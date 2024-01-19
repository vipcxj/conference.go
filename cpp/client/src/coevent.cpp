#include "cfgo/coevent.hpp"
#include <atomic>

namespace cfgo {

    bool co_event::trigger() {
        bool handler = false;
        {
            const std::lock_guard lock(m_mutex);
            if (--m_times == 0) {
                m_exception = std::current_exception();
                if (m_handler)
                {
                    handler = true;
                } 
                m_done = true;
            }
        }
        if (handler)
        {
            m_handler(m_exception);
        }
        return handler;
    }

    inline void invoke_handler(co_event::handler_type&& handler, std::exception_ptr&& e) {
        auto executor = asio::get_associated_executor(handler);
        auto alloc = asio::get_associated_allocator(
            handler, asio::recycling_allocator<void>()
        );
        asio::dispatch(
            executor,
            asio::bind_allocator(alloc, [handler = std::move(handler), e = std::move(e)]() mutable {
                std::move(handler)(e);
            })
        );
    }

    asio::awaitable<void> co_event::await(std::chrono::nanoseconds timeout) {
        return asio::async_initiate<decltype(asio::use_awaitable), void(std::exception_ptr)>([weak_self = weak_from_this()](auto&& handler) -> void {
            if (auto self = weak_self.lock())
            {
                bool invoke = false;
                {
                    std::lock_guard lock(self->m_mutex);
                    if (self->m_done)
                    {
                        invoke = true;
                    } else {
                        self->m_handler = [handler = std::move(handler)](std::exception_ptr e) mutable {
                            invoke_handler(std::move(handler), std::move(e));
                        };
                    }
                }
                if (invoke)
                {
                    invoke_handler(std::move(handler), std::move(self->m_exception));
                }
                
            }
        }, asio::use_awaitable);
    }
}