#include "impl/track.hpp"
#include "cpptrace/cpptrace.hpp"
#include "spdlog/spdlog.h"
#ifdef CFGO_SUPPORT_GSTREAMER
#include "gst/sdp/sdp.h"
#endif

#include <tuple>

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
            track->onOpen(std::bind(&Track::on_track_open, this));
            track->onClosed(std::bind(&Track::on_track_closed, this));
            track->onError(std::bind(&Track::on_track_error, this, std::placeholders::_1));
            m_inited = true;
        }

        uint32_t Track::makesure_min_seq()
        {
            if (m_rtp_cache.empty() && m_rtcp_cache.empty())
            {
                return 0xffffffff;
            }
            else if (m_rtp_cache.empty())
            {
                auto min_seq = m_rtcp_cache.front().first;
                if (min_seq == 0)
                {
                    m_rtcp_cache.pop_front();
                    return makesure_min_seq();
                }
                else
                {
                    return min_seq;
                }
            }
            else if (m_rtcp_cache.empty())
            {
                auto min_seq = m_rtp_cache.front().first;
                if (min_seq == 0)
                {
                    m_rtp_cache.pop_front();
                    return makesure_min_seq();
                }
                else
                {
                    return min_seq;
                }
            }
            else
            {
                auto min_seq_rtp = m_rtp_cache.front().first;
                auto min_seq_rtcp = m_rtcp_cache.front().first;
                if (min_seq_rtp < min_seq_rtcp)
                {
                    if (min_seq_rtp == 0)
                    {
                        m_rtp_cache.pop_front();
                        return makesure_min_seq();
                    }
                    else
                    {
                        return min_seq_rtp;
                    }
                }
                else
                {
                    if (min_seq_rtcp == 0)
                    {
                        m_rtcp_cache.pop_front();
                        return makesure_min_seq();
                    }
                    else
                    {
                        return min_seq_rtcp;
                    }
                }
            }
        }

        void Track::on_track_msg(rtc::message_variant msg) {
            if (std::holds_alternative<rtc::binary>(msg))
            {
                auto && data = std::get<rtc::binary>(msg);
                MsgBuffer & cache = rtc::IsRtcp(data) ? m_rtcp_cache : m_rtp_cache;
                std::lock_guard g(m_lock);
                if (m_seq == 0xffffffff)
                {
                    auto offset = makesure_min_seq();
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
                std::ignore = m_msg_notify.try_write();
            }
        }

        void Track::on_track_open()
        {
            if (!m_open_notify.try_write())
            {
                spdlog::warn("[Track::on_track_open] This should not happen.");
            }
        }

        void Track::on_track_closed()
        {
            if (!m_closed_notify.try_write())
            {
                spdlog::warn("[Track::on_track_closed] This should not happen.");
            }
        }

        void Track::on_track_error(std::string error)
        {
            spdlog::error(error);
        }

        auto Track::await_open_or_closed(close_chan close_ch) -> asio::awaitable<bool>
        {
            if (track->isOpen() || track->isClosed())
            {
                co_return true;
            }
            if (is_valid_close_chan(close_ch))
            {
                auto res = co_await cfgo::select(
                    close_ch,
                    asiochan::ops::read(m_open_notify, m_closed_notify)
                );
                if (!res)
                {
                    co_return false;
                }
                else if (res.received_from(m_open_notify))
                {
                    if (!m_open_notify.try_write())
                    {
                        spdlog::warn("[Track::await_open_or_closed:1] This should not happen.");
                    }
                }
                else
                {
                    if (!m_closed_notify.try_write())
                    {
                        spdlog::warn("[Track::await_open_or_closed:2] This should not happen.");
                    }
                }
                co_return true;
            }
            else
            {
                auto res = co_await asiochan::select(
                    asiochan::ops::read(m_open_notify, m_closed_notify)
                );
                if (res.received_from(m_open_notify))
                {
                    if (!m_open_notify.try_write())
                    {
                        spdlog::warn("[Track::await_open_or_closed:3] This should not happen.");
                    }
                }
                else
                {
                    if (!m_closed_notify.try_write())
                    {
                        spdlog::warn("[Track::await_open_or_closed:4] This should not happen.");
                    }
                }
                co_return true;
            }
        }

        auto Track::await_msg(cfgo::Track::MsgType msg_type, close_chan close_ch) -> asio::awaitable<cfgo::Track::MsgPtr>
        {
            if (!m_inited)
            {
                throw cpptrace::logic_error("Before call await_msg, call prepare_track at first.");
            }
            auto msg_ptr = receive_msg(msg_type);
            if (msg_ptr)
            {
                co_return msg_ptr;
            }
            if (is_valid_close_chan(close_ch) && close_ch.is_closed())
            {
                co_return nullptr;
            }
            if (!co_await await_open_or_closed(close_ch))
            {
                co_return nullptr;
            }
            if (is_valid_close_chan(close_ch) && close_ch.is_closed())
            {
                co_return nullptr;
            }
            do
            {
                if (is_valid_close_chan(close_ch))
                {
                    auto res = co_await cfgo::select(
                        close_ch,
                        asiochan::ops::read(m_msg_notify, m_closed_notify)
                    );
                    if (!res)
                    {
                        co_return nullptr;
                    }
                    else if (res.received_from(m_closed_notify))
                    {
                        if (!m_closed_notify.try_write())
                        {
                            spdlog::warn("[Track::await_msg:1] This should not happen.");
                        }
                    }
                }
                else
                {
                    auto res = co_await asiochan::select(
                        asiochan::ops::read(m_msg_notify, m_closed_notify)
                    );
                    if (res.received_from(m_closed_notify))
                    {
                        if (!m_closed_notify.try_write())
                        {
                            spdlog::warn("[Track::await_msg:2] This should not happen.");
                        }
                    }
                }

                auto msg_ptr = receive_msg(msg_type);
                if (msg_ptr)
                {
                    co_return msg_ptr;
                }
                if (track->isClosed())
                {
                    co_return nullptr;
                }
            } while (true);
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
