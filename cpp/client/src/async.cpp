#include "cfgo/async.hpp"
#include "cfgo/defer.hpp"

namespace cfgo
{
    close_chan INVALID_CLOSE_CHAN{};

    cancelable<void> make_resolved() {
        return cancelable<void>(false);
    }

    cancelable<void> make_canceled() {
        return cancelable<void>(true);
    }

    close_chan make_timeout(asio::execution::executor auto executor, const duration_t& dur) {
        auto timeout = close_chan{};
        asio::co_spawn(
            executor,
            [=]() mutable -> asio::awaitable<void> {
                auto timer = asio::steady_timer{executor};
                timer.expires_after(dur);
                co_await timer.async_wait(asio::use_awaitable);
                co_await timeout.write();
            },
            asio::detached
        );
        return timeout;
    }

    auto make_timeout(const duration_t& dur) -> asio::awaitable<close_chan> {
        auto executor = co_await asio::this_coro::executor;
        co_return make_timeout(executor, dur);
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

    struct CloseSignalState
    {
        bool m_closed = false;
        std::mutex m_mutex;
        AsyncMutex m_a_read_mutex;
    };

    CloseSignal::CloseSignal(): PC(), m_state(std::make_shared<CloseSignalState>())
    {}

    auto CloseSignal::try_read() -> bool
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
        if (m_state->m_closed)
        {
            co_return;
        }
        auto executor = co_await asio::this_coro::executor;
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
            if (!m_state->m_closed)
            {
                m_state->m_closed = true;
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

} // namespace cfgo
