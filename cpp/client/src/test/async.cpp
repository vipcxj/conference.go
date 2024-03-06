#include "cfgo/async.hpp"
#include "asio.hpp"
#include "gtest/gtest.h"
#include <chrono>
#include <thread>

void do_async(std::function<asio::awaitable<void>()> func, bool wait = false) {
    auto tp = std::make_shared<asio::thread_pool>();
    if (wait)
    {
        auto res = asio::co_spawn(*tp, func, asio::use_future);
        res.get();
    }
    else
    {
        asio::co_spawn(*tp, [tp, func]() -> asio::awaitable<void> {
            co_await func();
            co_return;
        }, asio::detached);
    }
}

TEST(Chan, CloseChan) {
    using namespace cfgo;
    close_chan close_ch;
    EXPECT_TRUE(is_valid_close_chan(close_ch));
    EXPECT_FALSE(is_valid_close_chan(INVALID_CLOSE_CHAN));
}

TEST(Chan, WaitTimeout) {
    using namespace cfgo;
    bool done = false;
    do_async([&done]() -> asio::awaitable<void> {
        co_await wait_timeout(std::chrono::milliseconds{500});
        done = true;
        co_return;
    }, false);
    std::this_thread::sleep_for(std::chrono::milliseconds{1600});
    EXPECT_TRUE(done);
}

TEST(Chan, MakeTimeout) {
    using namespace cfgo;
    bool done = false;
    do_async([&done]() -> asio::awaitable<void> {
        auto executor = co_await asio::this_coro::executor;
        auto timeout = cfgo::make_timeout(executor, std::chrono::milliseconds{500});
        co_await timeout.read();
        done = true;
        co_return;
    });
    std::this_thread::sleep_for(std::chrono::milliseconds{600});
    EXPECT_TRUE(done);
}

TEST(Chan, ChanRead) {
    using namespace cfgo;
    do_async([]() -> asio::awaitable<void> {
        asiochan::channel<int> ch{};
        do_async([ch]() mutable -> asio::awaitable<void> {
            co_await wait_timeout(std::chrono::milliseconds{100});
            co_await ch.write(1);
            co_return;
        });
        int res = 0;
        do_async([ch, &res]() mutable -> asio::awaitable<void> {
            auto res1 = co_await chan_read<int>(ch);
            EXPECT_FALSE(res1.is_canceled());
            EXPECT_EQ(res1.value(), 1);
            res = 1;
        });
        co_await wait_timeout(std::chrono::milliseconds{200});
        EXPECT_EQ(res, 1);

        bool canceled = false;
        do_async([ch]() mutable -> asio::awaitable<void> {
            co_await wait_timeout(std::chrono::milliseconds{200});
            co_await ch.write(1);
            co_return;
        });
        do_async([ch, &canceled]() mutable -> asio::awaitable<void> {
            auto timeout = co_await cfgo::make_timeout(std::chrono::milliseconds{100});
            auto res2 = co_await chan_read<int>(ch, timeout);
            EXPECT_TRUE(res2.is_canceled());
            canceled = true;
        });
        co_await wait_timeout(std::chrono::milliseconds{300});
        EXPECT_TRUE(canceled);

    }, true);
}

int main(int argc, char **argv) {
    testing::InitGoogleTest(&argc, argv);
    return RUN_ALL_TESTS();
}