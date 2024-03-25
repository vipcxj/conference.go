#include "cfgo/async.hpp"
#include "cfgo/defer.hpp"
#include "cfgo/utils.hpp"
#include "cpptrace/cpptrace.hpp"
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

    struct CloseSignalState : public std::enable_shared_from_this<CloseSignalState>
    {
        bool m_closed = false;
        std::mutex m_mutex;
        AsyncMutex m_a_read_mutex;
        duration_t m_timeout = duration_t {0};
        std::shared_ptr<asio::steady_timer> m_timer;
        std::list<asiochan::channel<void, 1>> m_waiters;

        void init_timer(asio::execution::executor auto executor, CloseSignal * s);
    };

    void CloseSignalState::init_timer(asio::execution::executor auto executor, CloseSignal * s)
    {
        std::cout << "close_ch: init_timer" << std::endl;
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
        std::cout << "close_ch: create timer" << std::endl;
        if (m_timeout != duration_t {0})
        {
            m_timer->expires_after(m_timeout);
            asio::co_spawn(executor, [s = *s]() mutable -> asio::awaitable<void> {
                co_await s.m_state->m_timer->async_wait(asio::use_awaitable);
                bool cancelled = false;
                {
                    std::lock_guard lock(s.m_state->m_mutex);
                    if (s.m_state->m_timeout == duration_t {0})
                    {
                        cancelled = true;
                    }
                }
                if (cancelled)
                {
                    co_return;
                }
                s.close();
            }, asio::detached);
        }
    }

    CloseSignal::CloseSignal(): m_state(std::make_shared<CloseSignalState>())
    {}

    auto CloseSignal::init_timer() -> asio::awaitable<void>
    {
        auto executor = co_await asio::this_coro::executor;
        m_state->init_timer(executor, this);
    }

    auto CloseSignal::get_waiter() -> std::optional<Waiter>
    {
        if (m_state->m_closed)
        {
            return std::nullopt;
        }
        std::lock_guard lock(m_state->m_mutex);
        if (m_state->m_closed)
        {
            return std::nullopt;
        }
        Waiter waiter {};
        m_state->m_waiters.push_back(waiter);
        return waiter;
    }

    bool CloseSignal::is_closed() const noexcept
    {
        std::lock_guard lock(m_state->m_mutex);
        return m_state->m_closed;
    }

    void CloseSignal::close()
    {
        if (m_state->m_closed)
        {
            return;
        }
        std::lock_guard lock(m_state->m_mutex);
        if (m_state->m_closed)
        {
            return;
        }
        m_state->m_closed = true;
        m_state->m_timeout = duration_t {0};
        m_state->m_timer->cancel();
        while (!m_state->m_waiters.empty())
        {
            auto && waiter = m_state->m_waiters.front();
            if (!waiter.try_write())
            {
                throw cpptrace::logic_error(cfgo::THIS_IS_IMPOSSIBLE);
            }
            m_state->m_waiters.pop_front();
        }
    }

    void CloseSignal::set_timeout(const duration_t& dur)
    {
        if (m_state->m_closed)
        {
            return;
        }
        std::lock_guard lock(m_state->m_mutex);
        if (m_state->m_timeout == dur)
        {
            return;
        }
        duration_t old_timeout = m_state->m_timeout;
        m_state->m_timeout = dur;
        if (m_state->m_timer)
        {
            if (dur == duration_t {0})
            {
                m_state->m_timer->cancel();
            }
            else if (old_timeout == duration_t {0})
            {
                m_state->m_timer->expires_after(dur);
                asio::co_spawn(m_state->m_timer->get_executor(), [self = *this]() mutable -> asio::awaitable<void> {
                    co_await self.m_state->m_timer->async_wait(asio::use_awaitable);
                    bool cancelled = false;
                    {
                        std::lock_guard lock(self.m_state->m_mutex);
                        if (self.m_state->m_timeout == duration_t {0})
                        {
                            cancelled = true;
                        }
                    }
                    if (cancelled)
                    {
                        co_return;
                    }
                    self.close();
                }, asio::detached);
            }
            else
            {
                m_state->m_timer->expires_at(m_state->m_timer->expiry() - old_timeout + dur);
            }
        }
    }

} // namespace cfgo
