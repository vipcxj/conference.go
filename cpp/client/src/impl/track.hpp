#ifndef _CFGO_TRACK_IMPL_HPP_
#define _CFGO_TRACK_IMPL_HPP_

#include <string>
#include <memory>
#include <map>
#include <deque>
#include <mutex>
#include "cfgo/track.hpp"
#include "boost/circular_buffer.hpp"

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
            boost::circular_buffer<cfgo::Track::MsgPtr> m_msg_cache;

            Track(const msg_ptr& msg, int cache_capicity);

            void prepare_track();
            void on_track_msg(rtc::message_variant data);
            cfgo::Track::MsgPtr receive_msg();
        };
    } // namespace impl
    
} // namespace name

#endif