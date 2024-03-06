#ifndef _CFGO_TRACK_HPP_
#define _CFGO_TRACK_HPP_

#include <string>
#include <memory>
#include "cfgo/alias.hpp"
#include "cfgo/utils.hpp"
#include "rtc/track.hpp"
#include "asio/awaitable.hpp"

namespace rtc
{
    struct Track;
} // namespace rtc

namespace cfgo
{
    namespace impl
    {
        struct Track;
    } // namespace impl
    
    constexpr int DEFAULT_TRACK_CACHE_CAPICITY = 16;
    
    struct Track : ImplBy<impl::Track>
    {
        using Ptr = std::shared_ptr<Track>;
        using MsgPtr = std::shared_ptr<rtc::message_variant>;

        Track(const msg_ptr & msg, int cache_capicity = DEFAULT_TRACK_CACHE_CAPICITY);

        const std::string& type() const noexcept;
        const std::string& pub_id() const noexcept;
        const std::string& global_id() const noexcept;
        const std::string& bind_id() const noexcept;
        const std::string& rid() const noexcept;
        const std::string& stream_id() const noexcept;
        std::map<std::string, std::string> & labels() noexcept;
        const std::map<std::string, std::string> & labels() const noexcept;
        std::shared_ptr<rtc::Track> & track() noexcept;
        const std::shared_ptr<rtc::Track> & track() const noexcept;
        MsgPtr receive_msg() const;
    };
    
} // namespace name

#endif