#ifndef _CFGO_TRACK_IMPL_HPP_
#define _CFGO_TRACK_IMPL_HPP_

#include <string>
#include <memory>
#include <map>
#include <deque>
#include <mutex>
#include <cstdint>
#include "cfgo/config/configuration.h"
#include "cfgo/track.hpp"
#include "impl/client.hpp"
#include "boost/circular_buffer.hpp"
#ifdef CFGO_SUPPORT_GSTREAMER
#include "gst/sdp/sdp.h"
#endif

namespace rtc
{
    class Track;
} // namespace rtc

namespace cfgo
{
    namespace impl
    {
        struct Track
        {
            using Ptr = std::shared_ptr<Track>;
            using MsgBuffer = boost::circular_buffer<std::pair<int, cfgo::Track::MsgPtr>>;
            std::string type;
            std::string pubId;
            std::string globalId;
            std::string bindId;
            std::string rid;
            std::string streamId;
            std::map<std::string, std::string> labels;
            std::shared_ptr<rtc::Track> track;

            bool m_inited;
            std::mutex m_lock;
            MsgBuffer m_rtp_cache;
            MsgBuffer m_rtcp_cache;
            uint32_t m_seq;
            std::shared_ptr<Client> m_client;
            asiochan::channel<void, 1> m_msg_notify;
            asiochan::channel<void, 1> m_open_notify;
            asiochan::channel<void, 1> m_closed_notify;
            #ifdef CFGO_SUPPORT_GSTREAMER
            GstSDPMedia *m_gst_media;
            #endif

            Track(const msg_ptr& msg, int cache_capicity);
            ~Track();

            uint32_t makesure_min_seq();
            void prepare_track();
            void on_track_msg(rtc::message_variant data);
            void on_track_open();
            void on_track_closed();
            void on_track_error(std::string error);
            auto await_open_or_closed(close_chan close_ch) -> asio::awaitable<bool>;
            cfgo::Track::MsgPtr receive_msg(cfgo::Track::MsgType msg_type);
            auto await_msg(cfgo::Track::MsgType msg_type, close_chan close_ch) -> asio::awaitable<cfgo::Track::MsgPtr>;
            void bind_client(std::shared_ptr<Client> client);
            void * get_gst_caps(int pt) const;
        };
    } // namespace impl
    
} // namespace name

#endif