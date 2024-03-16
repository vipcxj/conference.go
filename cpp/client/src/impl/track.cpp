#include "impl/track.hpp"
#include "cpptrace/cpptrace.hpp"
#include "spdlog/spdlog.h"
#ifdef CFGO_SUPPORT_GSTREAMER
#include "gst/sdp/sdp.h"
#endif

namespace cfgo
{
    namespace impl
    {
        Track::Track(const msg_ptr & msg, int cache_capicity)
        : m_rtp_cache(cache_capicity), m_rtcp_cache(cache_capicity), m_inited(false), m_seq(0)
        #ifdef CFGO_SUPPORT_GSTREAMER
        , m_gst_media(nullptr)
        #endif
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

        Track::~Track()
        {
            #ifdef CFGO_SUPPORT_GSTREAMER
            if (m_gst_media)
            {
                gst_sdp_media_free(m_gst_media);
            }
            #endif
        }

        #ifdef CFGO_SUPPORT_GSTREAMER
        const GstSDPMedia * get_media_from_sdp(GstSDPMessage *sdp, const char* mid)
        {
            auto media_len = gst_sdp_message_medias_len(sdp);
            for (size_t i = 0; i < media_len; i++)
            {
                auto media = gst_sdp_message_get_media(sdp, i);
                auto media_id = gst_sdp_media_get_attribute_val(media, "mid");
                if (!strcmp(mid, media_id))
                {
                    return media;
                }
            }
            return nullptr;
        }
        #endif

        void Track::bind_client(std::shared_ptr<Client> client)
        {
            if (!m_client)
            {
                m_client = client;
                #ifdef CFGO_SUPPORT_GSTREAMER
                auto mid = track->mid();
                auto sdp = client->m_gst_sdp;
                auto media = get_media_from_sdp(sdp, mid.c_str());
                if (!media)
                {
                    throw cpptrace::runtime_error("unable to find the media from sdp message with mid " + mid);
                }
                auto ret = gst_sdp_media_copy(media, &m_gst_media);
                if (ret != GstSDPResult::GST_SDP_OK)
                {
                    throw cpptrace::runtime_error("unable to clone the media from sdp message with mid " + mid);
                }
                #endif
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

        void Track::on_track_msg(rtc::message_variant msg) {
            if (std::holds_alternative<rtc::binary>(msg))
            {
                auto && data = std::get<rtc::binary>(msg);
                MsgBuffer & cache = rtc::IsRtcp(data) ? m_rtcp_cache : m_rtp_cache;
                std::lock_guard g(m_lock);
                if (m_seq == 0xffffffff)
                {
                    auto offset = 0xffffffff - m_rtcp_cache.capacity() - m_rtp_cache.capacity();
                    for (auto &&v : m_rtcp_cache)
                    {
                        v.first -= offset;
                    }
                    for (auto &&v : m_rtp_cache)
                    {
                        v.first -= offset;
                    }
                    m_seq -= offset;
                }
                cache.push_back(std::make_pair(m_seq++, std::make_unique<rtc::binary>(data)));
            }
        }

        cfgo::Track::MsgPtr Track::receive_msg(cfgo::Track::MsgType msg_type) {
            if (!m_inited)
            {
                throw cpptrace::logic_error("Before call receive_msg, call prepare_track at first.");
            }

            std::lock_guard g(m_lock);
            cfgo::Track::MsgPtr msg_ptr;
            if (msg_type == cfgo::Track::MsgType::ALL)
            {
                if (m_rtp_cache.empty() && m_rtcp_cache.empty())
                {
                    return cfgo::Track::MsgPtr();
                }
                else if (m_rtp_cache.empty())
                {
                    m_rtcp_cache.front().second.swap(msg_ptr);
                    m_rtcp_cache.pop_front();
                }
                else if (m_rtcp_cache.empty())
                {
                    m_rtp_cache.front().second.swap(msg_ptr);
                    m_rtp_cache.pop_front();
                }
                else
                {
                    auto & rtp = m_rtp_cache.front();
                    auto & rtcp = m_rtcp_cache.front();
                    if (rtp.first > rtcp.first)
                    {
                        rtcp.second.swap(msg_ptr);
                        m_rtcp_cache.pop_front();
                    }
                    else
                    {
                        rtp.second.swap(msg_ptr);
                        m_rtp_cache.pop_front();
                    }
                }
            }
            else if (msg_type == cfgo::Track::MsgType::RTP)
            {
                if (m_rtp_cache.empty())
                {
                    return cfgo::Track::MsgPtr();
                }
                m_rtp_cache.front().second.swap(msg_ptr);
                m_rtp_cache.pop_front();
            }
            else
            {
                if (m_rtcp_cache.empty())
                {
                    return cfgo::Track::MsgPtr();
                }
                m_rtcp_cache.front().second.swap(msg_ptr);
                m_rtcp_cache.pop_front();
            }
            return msg_ptr;
        }

        void * Track::get_gst_caps(int pt) const
        {
#ifdef CFGO_SUPPORT_GSTREAMER
            if (!m_gst_media)
            {
                throw cpptrace::logic_error("No gst sdp media found, please call bind_client at first.");
            }
            auto caps = gst_sdp_media_get_caps_from_media(m_gst_media, pt);
            gst_sdp_message_attributes_to_caps(m_client->m_gst_sdp, caps);
            gst_sdp_media_attributes_to_caps(m_gst_media, caps);
            auto s = gst_caps_get_structure(caps, 0);
            gst_structure_set_name(s, "application/x-rtp");
            if (!g_strcmp0 (gst_structure_get_string (s, "encoding-name"), "ULPFEC"))
                gst_structure_set (s, "is-fec", G_TYPE_BOOLEAN, TRUE, NULL);
            return caps;
#else
            throw cpptrace::logic_error("The gstreamer support is disabled, so to_gst_caps method is not supported. Please enable gstreamer support by set cmake GSTREAMER_SUPPORT option to ON.");
#endif
        }
    } // namespace impl
    
} // namespace cfgo
