#include "impl/track.hpp"
#include "cpptrace/cpptrace.hpp"

namespace cfgo
{
    namespace impl
    {
        Track::Track(const msg_ptr & msg, int cache_capicity): m_msg_cache(cache_capicity), m_inited(false)
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

        void Track::prepare_track() {
            if (!track)
            {
                throw cpptrace::logic_error("Before call receive_msg, a valid rtc::track should be set.");
            }
            track->onMessage(std::bind(&Track::on_track_msg, this, std::placeholders::_1));
            m_inited = true;
        }

        void Track::on_track_msg(rtc::message_variant data) {
            std::lock_guard g(m_lock);
            auto && ptr = std::make_shared<rtc::message_variant>(data);
            m_msg_cache.push_back(ptr);
        }

        cfgo::Track::MsgPtr Track::receive_msg() {
            if (!m_inited)
            {
                throw cpptrace::logic_error("Before call receive_msg, call prepare_track at first.");
            }
            std::lock_guard g(m_lock);
            if (m_msg_cache.empty())
            {
                return std::shared_ptr<rtc::message_variant>();
            }
            else
            {
                auto && value = m_msg_cache.front();
                m_msg_cache.pop_front();
                return value;
            }
        }
    } // namespace impl
    
} // namespace cfgo
