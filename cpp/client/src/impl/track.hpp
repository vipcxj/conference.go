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
            #ifdef CFGO_SUPPORT_GSTREAMER
            GstSDPMedia *m_gst_media;
            #endif

            Track(const msg_ptr& msg, int cache_capicity);
            ~Track();

            void prepare_track();
            void on_track_msg(rtc::message_variant data);
            cfgo::Track::MsgPtr receive_msg(cfgo::Track::MsgType msg_type);
            void bind_client(std::shared_ptr<Client> client);
            void * get_gst_caps(int pt) const;
        };
    } // namespace impl
    
} // namespace name

#endif