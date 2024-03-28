#include "cfgo/async.hpp"
#include "cfgo/defer.hpp"
#include "cfgo/utils.hpp"
#include "cpptrace/cpptrace.hpp"
#include "spdlog/spdlog.h"
#include <list>
#include <iostream>

namespace cfgo
{
    close_chan INVALID_CLOSE_CHAN{};

    cancelable<void> make_resolved() {
        return cancelable<void>(false);
    }

    cancelable<void> make_canceled() {
        return cancelable<void>(true);
    }

    close_chan make_timeout(const duration_t& dur) {
        auto timeout = close_chan{};
        timeout.set_timeout(dur);
        return timeout;
    }

    auto wait_timeout(const duration_t& dur) -> asio::awaitable<void> {
        auto executor = co_await asio::this_coro::executor;
        auto timer = asio::steady_timer{executor};
        timer.expires_after(dur);
        co_await timer.async_wait(asio::use_awaitable);
        co_return;
    }

    auto AsyncMutex::accquire(close_chan &close_chan) -> asio::awaitable<bool>
    {
        bool success = false;
        busy_chan ch{};
        bool ch_added = false;
        while (true)
        {
            {
                std::lock_guard g(m_mutex);
                if (!m_busy)
                {
                    m_busy = true;
                    success = true;
                    if (ch_added)
                    {
                        m_busy_chans.erase(std::remove(m_busy_chans.begin(), m_busy_chans.end(), ch), m_busy_chans.end());
                    }
                }
                else if (!ch_added)
                {
                    ch_added = true;
                    m_busy_chans.push_back(ch);
                }
            }
            if (success)
            {
                co_return true;
            }
            auto &&result = co_await chan_read<void>(ch, close_chan);
            if (result.is_canceled())
            {
                co_return false;
            }
        }
    }

    auto AsyncMutex::release() -> asio::awaitable<void>
    {
        auto executor = co_await asio::this_coro::executor;
        release(executor);
    }

    namespace detail
    {
        struct CloseSignalState : public std::enable_shared_from_this<CloseSignalState>
        {
            using Ptr = std::shared_ptr<CloseSignalState>;
            using Waiter = CloseSignal::Waiter;
            bool m_closed = false;
            bool m_is_timeout = false;
            std::mutex m_mutex;
            AsyncMutex m_a_read_mutex;
            duration_t m_timeout = duration_t {0};
            std::shared_ptr<asio::steady_timer> m_timer;
            std::list<Waiter> m_waiters;
            CloseSignalState * m_parent = nullptr;
            std::list<Ptr> m_children;

            ~CloseSignalState() noexcept;

            auto timer_task() -> asio::awaitable<void>;

            void init_timer(asio::execution::executor auto executor);

            auto get_waiter() -> std::optional<Waiter>;

            void _close(bool is_timeout);

            bool _close_no_except(bool is_timeout) noexcept;

            void close(bool is_timeout);

            bool close_no_except(bool is_timeout) noexcept;

            void set_timeout(const duration_t& dur);

            Ptr create_child();

            void remove_child(CloseSignalState * child);

            void unbind_parent() noexcept;
        };

        CloseSignalState::~CloseSignalState() noexcept
        {
            _close_no_except(false);
        }

        auto CloseSignalState::timer_task() -> asio::awaitable<void>
        {
            co_await m_timer->async_wait(asio::use_awaitable);
            bool cancelled = false;
            {
                std::lock_guard lock(m_mutex);
                if (m_timeout == duration_t {0})
                {
                    cancelled = true;
                }
            }
            if (cancelled)
            {
                co_return;
            }
            close(true);
        }

        void CloseSignalState::init_timer(asio::execution::executor auto executor)
        {
            if (m_closed || m_timer)
            {
                return;
            }

            std::lock_guard lock(m_mutex);
            if (m_closed || m_timer)
            {
                return;
            }
            m_timer = std::make_shared<asio::steady_timer>(executor);
            if (m_timeout != duration_t {0})
            {
                m_timer->expires_after(m_timeout);
                asio::co_spawn(executor, [weak_self = weak_from_this()]() mutable -> asio::awaitable<void> {
                    if (auto self = weak_self.lock())
                    {
                        co_await self->timer_task();
                    }
                }, asio::detached);
            }
        }

        auto CloseSignalState::get_waiter() -> std::optional<Waiter>
        {
            if (m_closed)
            {
                return std::nullopt;
            }
            std::lock_guard lock(m_mutex);
            if (m_closed)
            {
                return std::nullopt;
            }
            Waiter waiter {};
            m_waiters.push_back(waiter);
            return waiter;
        }

        void CloseSignalState::_close(bool is_timeout)
        {
            if (m_closed)
            {
                return;
            }
            m_closed = true;
            m_is_timeout = is_timeout;
            m_timeout = duration_t {0};
            if (m_timer)
            {
                m_timer->cancel();
            }
            while (!m_waiters.empty())
            {
                auto && waiter = m_waiters.front();
                if (!waiter.try_write())
                {
                    throw cpptrace::logic_error(cfgo::THIS_IS_IMPOSSIBLE);
                }
                m_waiters.pop_front();
            }
            if (m_parent != nullptr)
            {
                m_parent->remove_child(this);
            }
            while (!m_children.empty())
            {
                auto child = m_children.front();
                child->unbind_parent();
                child->close_no_except(is_timeout);
                m_children.pop_front();
            }
        }

        bool CloseSignalState::_close_no_except(bool is_timeout) noexcept
        {
            try
            {
                _close(false);
                return true;
            }
            catch(...)
            {
                spdlog::error(cfgo::what());
            }
            return false;
        }

        void CloseSignalState::close(bool is_timeout)
        {
            if (m_closed)
            {
                return;
            }
            std::lock_guard lock(m_mutex);
            _close(is_timeout);
        }

        bool CloseSignalState::close_no_except(bool is_timeout) noexcept
        {
            if (m_closed)
            {
                return true;
            }
            std::lock_guard lock(m_mutex);
            return _close_no_except(is_timeout);
        }

        void CloseSignalState::set_timeout(const duration_t& dur)
        {
            if (m_closed)
            {
                return;
            }
            std::lock_guard lock(m_mutex);
            if (m_timeout == dur)
            {
                return;
            }
            duration_t old_timeout = m_timeout;
            m_timeout = dur;
            if (m_timer)
            {
                if (dur == duration_t {0})
                {
                    m_timer->cancel();
                }
                else if (old_timeout == duration_t {0})
                {
                    m_timer->expires_after(dur);
                    asio::co_spawn(m_timer->get_executor(), [weak_self = weak_from_this()]() mutable -> asio::awaitable<void> {
                        if (auto self = weak_self.lock())
                        {
                            co_await self->timer_task();
                        }
                    }, asio::detached);
                }
                else
                {
                    m_timer->expires_at(m_timer->expiry() - old_timeout + dur);
                }
            }
        }

        auto CloseSignalState::create_child() -> Ptr
        {
            std::lock_guard lock(m_mutex);
            auto child = std::make_shared<CloseSignalState>();
            child->m_parent = this;
            m_children.push_back(child);
            return child;
        }

        void CloseSignalState::remove_child(CloseSignalState * child)
        {
            std::lock_guard lock(m_mutex);
            m_children.erase(
                std::remove_if(m_children.begin(), m_children.end(), [child](auto && v) { return v.get() == child; }),
                m_children.end()
            );
        }

        void CloseSignalState::unbind_parent() noexcept
        {
            std::lock_guard lock(m_mutex);
            m_parent = nullptr;
        }
    } // namespace detail

    CloseSignal::CloseSignal(const std::shared_ptr<detail::CloseSignalState> & state): m_state(state)
    {}

    CloseSignal::CloseSignal(std::shared_ptr<detail::CloseSignalState> && state): m_state(std::move(state))
    {}

    CloseSignal::CloseSignal(): m_state(std::make_shared<detail::CloseSignalState>())
    {}

    auto CloseSignal::init_timer() -> asio::awaitable<void>
    {
        auto executor = co_await asio::this_coro::executor;
        m_state->init_timer(executor);
    }

    auto CloseSignal::get_waiter() -> std::optional<Waiter>
    {
        return m_state->get_waiter();
    }

    bool CloseSignal::is_closed() const noexcept
    {
        return m_state->m_closed;
    }

    bool CloseSignal::is_timeout() const noexcept
    {
        return m_state->m_is_timeout;
    }

    void CloseSignal::close()
    {
        m_state->close(false);
    }

    bool CloseSignal::close_no_except() noexcept
    {
        return m_state->close_no_except(false);
    }

    void CloseSignal::set_timeout(const duration_t& dur)
    {
        m_state->set_timeout(dur);
    }

    CloseSignal CloseSignal::create_child()
    {
        return m_state->create_child();
    }

    auto CloseSignal::await() -> asio::awaitable<bool>
    {
        co_await init_timer();
        if (auto waiter = get_waiter())
        {
            co_await waiter->read();
        }
        co_return !is_timeout();
    }

} // namespace cfgo
