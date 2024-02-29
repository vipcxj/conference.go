#include "impl/track.hpp"

namespace cfgo
{
    namespace impl
    {
        Track::Track(const msg_ptr & msg)
        {
            auto &&map = msg->get_map();
            if (auto &&mp = map["type"])
            {
                type = mp->get_string();
            }
            if (auto &&mp = map["pubId"])
            {
                pubId = mp->get_string();
            }
            if (auto &&mp = map["globalId"])
            {
                globalId = mp->get_string();
            }
            if (auto &&mp = map["bindId"])
            {
                bindId = mp->get_string();
            }
            if (auto &&mp = map["rid"])
            {
                rid = mp->get_string();
            }
            if (auto &&mp = map["streamId"])
            {
                streamId = mp->get_string();
            }
            if (auto &&mp = map["labels"])
            {
                for (auto &&[key, value] : mp->get_map())
                {
                    labels[key] = value->get_string();
                }
            }
        }
    } // namespace impl
    
} // namespace cfgo
