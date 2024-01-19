#pragma once
#ifndef _CFGO_COEVENT_HPP_
#define _CFGO_COEVENT_HPP_

#include <chrono>
#include <mutex>
#include <atomic>
#include "asio.hpp"
#include "cfgo/move_only_function.hpp"

namespace cfgo {

    class co_event : public std::enable_shared_from_this<co_event>
    {
    public:
        using handler_type = asio::async_result<std::decay<decltype(asio::use_awaitable)>::type, void(std::exception_ptr)>::handler_type;
        using ptr = std::shared_ptr<co_event>;
    private:
        int m_times;
        std::mutex m_mutex;
        std::atomic_bool m_done;
        std::exception_ptr m_exception;
        cfgo::unique_function<void(std::exception_ptr)> m_handler;
    protected:
        co_event(int times = 1): m_times(times), m_exception(), m_handler(), m_mutex(), m_done(false) {}
        co_event(const co_event&) = delete;
        const co_event& operator = (const co_event&) = delete;
    public:
        ~co_event() = default;
        inline static ptr create(int times = 1) {
            return std::shared_ptr<co_event>(new co_event(times));
        }
        bool trigger();
        asio::awaitable<void> await(std::chrono::nanoseconds timeout = std::chrono::nanoseconds{0});
    };
    
    
}

#endif