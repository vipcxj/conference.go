#include "cfgo/async.hpp"

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
        auto timeout = asiochan::channel<void>{};
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
} // namespace cfgo
