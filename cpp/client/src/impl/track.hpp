#ifndef _CFGO_TRACK_IMPL_HPP_
#define _CFGO_TRACK_IMPL_HPP_

#include <string>
#include <memory>
#include <map>
#include "cfgo/alias.hpp"

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

            Track(const msg_ptr& msg);
        };
    } // namespace impl
    
} // namespace name

#endif