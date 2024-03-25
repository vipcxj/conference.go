#include "cfgo/async.hpp"
#include "cfgo/defer.hpp"
#include "cfgo/utils.hpp"
#include "cpptrace/cpptrace.hpp"
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
                co_await s.write();
            }, asio::detached);
        }
    }

    CloseSignal::CloseSignal(): PC(), m_state(std::make_shared<CloseSignalState>())
    {}

    // void CloseSignal::clear_waiter()
    // {
    //     auto readers = shared_state().reader_list();
    //     while (auto node = readers.dequeue_first_available())
    //     {
    //         asiochan::detail::notify_waiter(*node);
    //     }
    //     auto writers = shared_state().writer_list();
    //     while (auto node = writers.dequeue_first_available())
    //     {
    //         asiochan::detail::notify_waiter(*node);
    //     }
    // }

    auto CloseSignal::try_read() -> bool
    {
        std::cout << "close_ch: try_read" << std::endl;
        if (m_state->m_closed)
        {
            return true;
        }
        std::lock_guard lock(m_state->m_mutex);
        if (m_state->m_closed)
        {
            return true;
        }
        else
        {
            return PC::try_read();
        }
    }

    auto CloseSignal::try_write() -> bool
    {
        if (m_state->m_closed)
        {
            return true;
        }
        std::lock_guard lock(m_state->m_mutex);
        if (m_state->m_closed)
        {
            return true;
        }
        if (PC::try_write())
        {
            m_state->m_closed = true;
            return true;
        }
        else
        {
            return false;
        }
    }

    auto CloseSignal::read() -> asio::awaitable<void>
    {
        std::cout << "close_ch: read" << std::endl;
        if (m_state->m_closed)
        {
            co_return;
        }
        auto executor = co_await asio::this_coro::executor;
        m_state->init_timer(executor, this);
        if (co_await m_state->m_a_read_mutex.accquire())
        {
            DEFER({
                m_state->m_a_read_mutex.release(executor);
            });
            if (m_state->m_closed)
            {
                co_return;
            }
            co_await PC::read();
            {
                std::lock_guard lock(m_state->m_mutex);
                if (!m_state->m_closed)
                {
                    m_state->m_closed = true;
                    // clear_waiter();
                }
            }
            co_return;
        }
    }

    auto CloseSignal::write() -> asio::awaitable<void>
    {
        if (m_state->m_closed)
        {
            co_return;
        }
        bool writed = true;
        {
            std::lock_guard lock(m_state->m_mutex);
            if (!m_state->m_closed)
            {
                m_state->m_closed = true;
                writed = PC::try_write();
            }
        }
        if (!writed)
        {

            co_await PC::write();
        }
        co_return;
    }

    bool CloseSignal::is_closed() const noexcept
    {
        return m_state->m_closed;
    }

    void CloseSignal::close()
    {
        if (!try_write())
        {
            throw cpptrace::logic_error(THIS_IS_IMPOSSIBLE);
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
        std::cout << "close_ch: set timeout to " << std::chrono::duration_cast<std::chrono::milliseconds>(dur).count() << " ms." << std::endl;
        m_state->m_timeout = dur;
        if (m_state->m_timer)
        {
            std::cout << "close_ch: timer exist." << std::endl;
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
                    co_await self.write();
                }, asio::detached);
            }
            else
            {
                m_state->m_timer->expires_at(m_state->m_timer->expiry() - old_timeout + dur);
            }
        }
    }

} // namespace cfgo
