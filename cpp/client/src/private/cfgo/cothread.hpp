#ifndef _CFGO_COTHREAD_HPP_
#define _CFGO_COTHREAD_HPP_

#include "asio.hpp"

template<typename R, asio::completion_token_for<void(std::exception_ptr, R)> CompletionToken>
auto make_awaitable(std::function<R()> fn, R invalid, CompletionToken&& token)
{
    auto init = [fn = std::move(fn), invalid = std::move(invalid)](
        asio::completion_handler_for<void(std::exception_ptr, R)> auto handler
    )
    {
        auto work = asio::make_work_guard(handler);
        std::thread(
            [fn = std::move(fn), invalid = std::move(invalid), handler = std::move(handler), work = std::move(work)]() mutable
            {
                // Get the handler's associated allocator. If the handler does not
                // specify an allocator, use the recycling allocator as the default.
                auto alloc = asio::get_associated_allocator(
                    handler, asio::recycling_allocator<void>()
                );

                try {
                    auto r = std::move(fn)();
                    asio::dispatch(
                        work.get_executor(),
                        asio::bind_allocator(alloc, [handler = std::move(handler), fn = std::move(fn), r = std::move(r)]() mutable {
                            std::move(handler)(std::current_exception(), std::move(r));
                        })
                    );
                } catch (...) {
                    auto e = std::current_exception();
                    asio::dispatch(
                        work.get_executor(),
                        asio::bind_allocator(alloc, [handler = std::move(handler), fn = std::move(fn), invalid = std::move(invalid), e = std::move(e)]() mutable {
                            std::move(handler)(e, std::move(invalid));
                        })
                    );
                }
            }
        ).detach();
    };
    return asio::async_initiate<CompletionToken, void(std::exception_ptr, R)>(
        init,
        token
    );
}

auto make_void_awaitable(std::function<void()> fn)
{
    return asio::async_compose<decltype(asio::use_awaitable), void()>(
        [&fn](auto&& self){
            std::thread(
                [self = std::move(self), fn = std::move(fn)]() mutable
                {
                    fn();
                    self.complete();
                }
            ).detach();
        },
        asio::use_awaitable
    );
}

#endif