#include "cfgo/async.hpp"
#include "cfgo/defer.hpp"
#include "cfgo/utils.hpp"
#include "spdlog/spdlog.h"
#include <list>
#include <chrono>

namespace cfgo
{
    close_chan INVALID_CLOSE_CHAN{};

    CancelError::CancelError(std::string&& message, Reason reason, bool trace) noexcept:
        cpptrace::exception_with_message(std::move(message), trace ? cpptrace::detail::get_raw_trace_and_absorb() : cpptrace::raw_trace{}),
        m_reason(reason),
        m_trace(trace)
    {}

    CancelError::CancelError(Reason reason, bool trace) noexcept: CancelError("", reason, trace)
    {}

    CancelError::CancelError(bool is_timeout, bool trace) noexcept: CancelError(is_timeout ? TIMEOUT : CANCEL, trace)
    {}

    CancelError::CancelError(const close_chan & close_ch, bool trace) noexcept:
        CancelError(
            close_ch.is_timeout() ? std::move(close_ch.get_timeout_reason()) : std::move(close_ch.get_close_reason()),
            close_ch.is_timeout() ? TIMEOUT : CANCEL,
            trace
        )
    {}

    const char* CancelError::what() const noexcept
    {
        return m_trace ? cpptrace::exception_with_message::what() : cpptrace::exception_with_message::message();
    }

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

    auto AsyncMutex::accquire(close_chan close_chan) -> asio::awaitable<bool>
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
            std::string m_close_reason;
            bool m_is_timeout = false;
            bool m_stop = false;
            std::mutex m_mutex;
            duration_t m_timeout = duration_t {0};
            duration_t m_stop_timeout = duration_t {0};
            std::string m_timeout_reason;
            std::shared_ptr<asio::steady_timer> m_timer;
            std::list<Waiter> m_waiters;
            std::list<Waiter> m_stop_waiters;
            CloseSignalState * m_parent = nullptr;
            std::list<Ptr> m_children;

            ~CloseSignalState() noexcept;

            auto timer_task() -> asio::awaitable<void>;

            void init_timer(asio::execution::executor auto executor);

            auto get_waiter() -> std::optional<Waiter>;

            void _close(bool is_timeout, std::string && reason);

            bool _close_no_except(bool is_timeout, std::string && reason) noexcept;

            void close(bool is_timeout, std::string && reason);

            bool close_no_except(bool is_timeout, std::string && reason) noexcept;

            void stop(bool stop_timer);

            void resume();

            auto get_stop_waiter() -> std::optional<Waiter>;

            void _set_timeout(const duration_t& dur, std::string && reason);

            void set_timeout(const duration_t& dur, std::string && reason);

            Ptr create_child();

            void remove_child(CloseSignalState * child);

            void unbind_parent() noexcept;
        };

        CloseSignalState::~CloseSignalState() noexcept
        {
            _close_no_except(false, "Destructor called.");
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
            close(true, std::string(m_timeout_reason));
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
                asio::co_spawn(executor, fix_async_lambda([self = shared_from_this()]() -> asio::awaitable<void> {
                    co_await self->timer_task();
                }), asio::detached);
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

        void CloseSignalState::_close(bool is_timeout, std::string && reason)
        {
            if (m_closed)
            {
                return;
            }
            m_closed = true;
            m_stop = false;
            m_close_reason = std::move(reason);
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
            while (!m_stop_waiters.empty())
            {
                auto && waiter = m_stop_waiters.front();
                if (!waiter.try_write())
                {
                    throw cpptrace::logic_error(cfgo::THIS_IS_IMPOSSIBLE);
                }
                m_stop_waiters.pop_front();
            }
            if (m_parent != nullptr)
            {
                m_parent->remove_child(this);
            }
            while (!m_children.empty())
            {
                auto child = m_children.front();
                child->unbind_parent();
                child->close_no_except(is_timeout, std::move(reason));
                m_children.pop_front();
            }
        }

        bool CloseSignalState::_close_no_except(bool is_timeout, std::string && reason) noexcept
        {
            try
            {
                _close(false, std::move(reason));
                return true;
            }
            catch(...)
            {
                spdlog::error(cfgo::what());
            }
            return false;
        }

        void CloseSignalState::close(bool is_timeout, std::string && reason)
        {
            if (m_closed)
            {
                return;
            }
            std::lock_guard lock(m_mutex);
            _close(is_timeout, std::move(reason));
        }

        bool CloseSignalState::close_no_except(bool is_timeout, std::string && reason) noexcept
        {
            if (m_closed)
            {
                return true;
            }
            std::lock_guard lock(m_mutex);
            return _close_no_except(is_timeout, std::move(reason));
        }

        void CloseSignalState::stop(bool stop_timer)
        {
            if (m_closed || m_stop)
            {
                return;
            }
            std::lock_guard lock(m_mutex);
            if (!m_closed && !m_stop)
            {
                m_stop = true;
                if (stop_timer)
                {
                    if (m_timer && m_timeout > duration_t {0})
                    {
                        auto now = std::chrono::steady_clock::now();
                        if (m_timer->expiry() > now)
                        {
                            m_stop_timeout = m_timer->expiry() - now;
                            _set_timeout(duration_t {0}, std::move(m_timeout_reason));
                        }
                    }
                    else
                    {
                        m_stop_timeout = m_timeout;
                        _set_timeout(duration_t {0}, std::move(m_timeout_reason));
                    }
                }
                for (auto && child : m_children)
                {
                    child->stop(stop_timer);
                }
            }
        }

        void CloseSignalState::resume()
        {
            if (!m_stop)
            {
                return;
            }
            std::lock_guard lock(m_mutex);
            if (m_stop)
            {
                m_stop = false;
                while (!m_stop_waiters.empty())
                {
                    auto waiter = m_stop_waiters.front();
                    if (!waiter.try_write())
                    {
                        throw cpptrace::logic_error(cfgo::THIS_IS_IMPOSSIBLE);
                    }
                    m_stop_waiters.pop_front();
                }
                if (m_stop_timeout > duration_t {0})
                {
                    _set_timeout(m_stop_timeout, std::move(m_timeout_reason));
                    m_stop_timeout = duration_t {0};
                }
                for (auto && child : m_children)
                {
                    child->resume();
                }
            }
        }

        auto CloseSignalState::get_stop_waiter() -> std::optional<Waiter>
        {
            if (!m_stop)
            {
                return std::nullopt;
            }
            std::lock_guard lock(m_mutex);
            if (!m_stop)
            {
                return std::nullopt;
            }
            Waiter waiter {};
            m_stop_waiters.push_back(waiter);
            return waiter;
        }

        void CloseSignalState::_set_timeout(const duration_t& dur, std::string&& reason)
        {
            if (m_closed)
            {
                return;
            }
            m_timeout_reason = std::move(reason);
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
                    asio::co_spawn(m_timer->get_executor(), fix_async_lambda([self = shared_from_this()]() -> asio::awaitable<void> {
                        co_await self->timer_task();
                    }), asio::detached);
                }
                else
                {
                    m_timer->expires_at(m_timer->expiry() - old_timeout + dur);
                }
            }
        }

        void CloseSignalState::set_timeout(const duration_t& dur, std::string&& reason)
        {
            if (m_closed)
            {
                return;
            }
            std::lock_guard lock(m_mutex);
            _set_timeout(dur, std::move(reason));
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

    auto CloseSignal::get_stop_waiter() -> std::optional<Waiter>
    {
        return m_state->get_stop_waiter();
    }

    bool CloseSignal::is_closed() const noexcept
    {
        return m_state->m_closed;
    }

    bool CloseSignal::is_timeout() const noexcept
    {
        return m_state->m_is_timeout;
    }

    void CloseSignal::close(std::string && reason)
    {
        m_state->close(false, std::move(reason));
    }

    bool CloseSignal::close_no_except(std::string && reason) noexcept
    {
        return m_state->close_no_except(false, std::move(reason));
    }

    void CloseSignal::set_timeout(const duration_t& dur, std::string && reason)
    {
        m_state->set_timeout(dur, std::move(reason));
    }

    duration_t CloseSignal::get_timeout() const noexcept
    {
        return m_state->m_timeout;
    }

    void CloseSignal::stop(bool stop_timer)
    {
        m_state->stop(stop_timer);
    }

    void CloseSignal::resume()
    {
        m_state->resume();
    }

    CloseSignal CloseSignal::create_child() const
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

    const char * CloseSignal::get_close_reason() const noexcept
    {
        return m_state->m_close_reason.c_str();
    }

    const char * CloseSignal::get_timeout_reason() const noexcept
    {
        return m_state->m_timeout_reason.c_str();
    }
    
} // namespace cfgo
