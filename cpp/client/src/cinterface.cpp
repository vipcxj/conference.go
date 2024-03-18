#include "cfgo/cinterface.h"
#include "cfgo/cbridge.hpp"
#include "cfgo/utils.hpp"
#include "cpptrace.hpp"
#include "spdlog/spdlog.h"
#include "asio/io_context.hpp"
#include "asio/thread_pool.hpp"
#include "asio/co_spawn.hpp"
#include "asio/associated_executor.hpp"
#include "asio/detached.hpp"

#include <exception>
#include <mutex>
#include <unordered_map>

#define CFGO_HANDLE_NOT_EXIST(who) "The " who " handle does not exist"

#define CFGO_DEFINE_CPP_UNWRAP(NAME, TYPE, MSG) \
    TYPE get_ ## NAME(int handle) \
    { \
        std::lock_guard lock(c_mutex); \
        if (auto it = NAME ## _map.find(handle); it != NAME ## _map.end()) \
        { \
            return it->second; \
        } \
        else \
        { \
            throw cpptrace::invalid_argument(CFGO_HANDLE_NOT_EXIST(MSG)); \
        } \
    }

#define CFGO_DEFINE_CPP_WRAP(NAME, TYPE) \
    int emplace_ ## NAME(TYPE ptr) \
    { \
        std::lock_guard lock(c_mutex); \
        int handle = ++last_handle; \
        NAME ## _map.emplace(std::make_pair(handle, ptr)); \
        return handle; \
    }

#define CFGO_DEFINE_CPP_RELEASE(NAME, MSG) \
    void erase_ ## NAME(int handle) \
    { \
        std::lock_guard lock(c_mutex); \
        if (NAME ## _map.erase(handle) == 0) \
        { \
            throw cpptrace::invalid_argument(CFGO_HANDLE_NOT_EXIST(MSG)); \
        } \
    }

namespace cfgo
{

    std::unordered_map<int, cfgo::Client::CtxPtr> execution_context_map;
    std::unordered_map<int, cfgo::close_chan_ptr> close_chan_map;
    std::unordered_map<int, cfgo::Client::Ptr> client_map;
    std::unordered_map<int, cfgo::Subscribation::Ptr> subscribation_map;
    std::unordered_map<int, cfgo::Track::Ptr> track_map;

    std::mutex c_mutex;
    int last_handle = 0;

    CFGO_DEFINE_CPP_UNWRAP(execution_context, Client::CtxPtr, "execution context")
    CFGO_DEFINE_CPP_WRAP(execution_context, Client::CtxPtr)
    CFGO_DEFINE_CPP_RELEASE(execution_context, "execution context")

    CFGO_DEFINE_CPP_UNWRAP(close_chan, close_chan_ptr, "close chan")
    CFGO_DEFINE_CPP_WRAP(close_chan, close_chan_ptr)
    CFGO_DEFINE_CPP_RELEASE(close_chan, "close chan")

    CFGO_DEFINE_CPP_UNWRAP(client, Client::Ptr, "client")
    CFGO_DEFINE_CPP_WRAP(client, Client::Ptr)
    CFGO_DEFINE_CPP_RELEASE(client, "client")

    CFGO_DEFINE_CPP_UNWRAP(subscribation, Subscribation::Ptr, "subscribation")
    CFGO_DEFINE_CPP_WRAP(subscribation, Subscribation::Ptr)
    CFGO_DEFINE_CPP_RELEASE(subscribation, "subscribation")

    CFGO_DEFINE_CPP_UNWRAP(track, Track::Ptr, "track")
    CFGO_DEFINE_CPP_WRAP(track, Track::Ptr)
    CFGO_DEFINE_CPP_RELEASE(track, "track")

    template<typename F>
    int c_wrap(F func) {
        try
        {
            return int(func());
        }
        catch(...)
        {
            spdlog::error(cfgo::what(std::current_exception()));
            return CFGO_ERR_FAILURE;
        }
    }

} // namespace name

CFGO_API int cfgo_execution_context_create_io_context()
{
    return cfgo::c_wrap([]() {
        return cfgo::emplace_execution_context(std::make_shared<asio::io_context>());
    });
}

CFGO_API int cfgo_execution_context_create_thread_pool(int n)
{
    return cfgo::c_wrap([n]() {
        return cfgo::emplace_execution_context(std::make_shared<asio::thread_pool>(n));
    });
}

CFGO_API int cfgo_execution_context_create_thread_pool_auto()
{
    return cfgo::c_wrap([]() {
        return cfgo::emplace_execution_context(std::make_shared<asio::thread_pool>());
    });
}

CFGO_API int cfgo_execution_context_release(int handle)
{
    return cfgo::c_wrap([handle]() {
        cfgo::erase_execution_context(handle);
        return CFGO_ERR_SUCCESS;
    });
}

CFGO_API int cfgo_close_chan_create()
{
    return cfgo::c_wrap([]() {
        return cfgo::emplace_close_chan(std::make_shared<cfgo::close_chan>());
    });
}

CFGO_API int cfgo_close_chan_close(int ctx_handle, int chan_handle)
{
    return cfgo::c_wrap([ctx_handle, chan_handle]() {
        auto ctx = cfgo::get_execution_context(ctx_handle);
        auto chan = cfgo::get_close_chan(chan_handle);
        asio::co_spawn(asio::get_associated_executor(ctx), chan->write(), asio::detached);
        return CFGO_ERR_SUCCESS;
    });
}
CFGO_API int cfgo_close_chan_release(int handle)
{
    return cfgo::c_wrap([handle]() {
        cfgo::erase_close_chan(handle);
        return CFGO_ERR_SUCCESS;
    });
}