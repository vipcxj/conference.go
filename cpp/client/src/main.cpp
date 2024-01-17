#include "asio/io_context.hpp"
#include "asio/steady_timer.hpp"
#include "asio/thread.hpp"
#include "asio/use_awaitable.hpp"
#include <chrono>
#include <iostream>
#include <thread>
#include <future>
#include "cfgo/cofuture.hpp"
#include "cfgo/cothread.hpp"

using namespace asio;

std::future<int> compute(as_coroutine) {
    int a = co_await std::async([] {
        std::cout << "waiting 3 sec..." << std::endl;
        std::this_thread::sleep_for(std::chrono::seconds(3));
        std::cout << "got 6." << std::endl;
        return 6;
    });
    int b = co_await std::async([] {
        std::cout << "waiting 3 sec..." << std::endl;
        std::this_thread::sleep_for(std::chrono::seconds(3));
        std::cout << "got 7." << std::endl;
        return 7; 
    });
    co_return a * b;
}
  
std::future<void> fail(as_coroutine) {
  throw std::runtime_error("bleah");
  co_return;
}

std::string what(const std::exception_ptr &eptr = std::current_exception())
{
    if (!eptr) { throw std::bad_exception(); }

    try { std::rethrow_exception(eptr); }
    catch (const std::exception &e)  { return e.what()   ; }
    catch (const std::string    &e)  { return e          ; }
    catch (const char           *e)  { return e          ; }
    catch (const std::error_code &e) { return e.message(); }
    catch (...)                      { return "who knows"; }
}

int main()
{
    io_context io_ctx;
    auto interval = std::chrono::seconds(1);
    steady_timer timer(io_ctx, interval);
    int times = 3;
    auto word = make_work_guard(io_ctx);
    std::cout << "main thread " << std::this_thread::get_id() << std::endl;
    co_spawn(io_ctx, [&times, &timer, interval]() -> awaitable<void> {
        std::cout << "1) tick" << std::endl;
        while (times-- > 0)
        {
            timer.expires_at(timer.expires_at() + interval);
            co_await timer.async_wait(use_awaitable);
            co_await make_awaitable<int>([]() -> int {
                std::cout << "1) waiting 1 sec...\n";
                std::this_thread::sleep_for(std::chrono::seconds(1));
                std::cout << "1) waited" << std::endl;
                return 0;
            }, 0, asio::use_awaitable);
            auto three = co_await make_awaitable<int>([]() -> int {
                return 3;
            }, 0, asio::use_awaitable);
            try
            {
                co_await make_awaitable<int>([]() -> int {
                    throw std::make_error_code(std::errc::address_in_use);
                }, 0, asio::use_awaitable);
            }
            catch(...)
            {
                std::cerr << "1) " << what() << '\n';
            }
            
        }
    }, detached);

    co_spawn(io_ctx, [&times, &timer, interval]() -> awaitable<void> {
        std::cout << "2) tick" << std::endl;
        while (times-- > 0)
        {
            timer.expires_at(timer.expires_at() + interval);
            co_await timer.async_wait(use_awaitable);
            co_await make_awaitable<int>([]() -> int {
                std::cout << "2) waiting 1 sec...\n";
                std::this_thread::sleep_for(std::chrono::seconds(1));
                std::cout << "2) waited" << std::endl;
                return 0;
            }, 0, asio::use_awaitable);
            auto three = co_await make_awaitable<int>([]() -> int {
                return 3;
            }, 0, asio::use_awaitable);
            try
            {
                co_await make_awaitable<int>([]() -> int {
                    throw std::make_error_code(std::errc::address_in_use);
                }, 0, asio::use_awaitable);
            }
            catch(...)
            {
                std::cerr << "2) " << what() << '\n';
            }
            
        }
    }, detached);

    io_ctx.run();
}