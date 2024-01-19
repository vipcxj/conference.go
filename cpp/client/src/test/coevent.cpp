#include "cfgo/coevent.hpp"
#include "asio.hpp"
#include <iostream>
#include <thread>

int main() {
    asio::io_context io_ctx;
    asio::co_spawn(io_ctx, [&io_ctx]() -> asio::awaitable<void> {
        auto event = cfgo::co_event::create();
        std::thread([event]() {
            std::this_thread::sleep_for(std::chrono::seconds{3});
            std::cout << "trigger" << std::endl;
            event->trigger();
        }).detach();
        std::cout << "before await" << std::endl;
        co_await event->await();
        std::cout << "awaited" << std::endl;
        co_return;
    }, asio::detached);
    io_ctx.run();
    return 0;
}