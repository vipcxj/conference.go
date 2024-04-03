#include "cfgo/gst/cfgosrc.hpp"
#include "cfgo/gst/gstcfgosrc_private_api.hpp"
#include "cfgo/gst/error.hpp"
#include "cfgo/gst/utils.hpp"
#include "cfgo/common.hpp"
#include "cfgo/cfgo.hpp"
#include "cfgo/defer.hpp"
#include "cfgo/fmt.hpp"
#include "spdlog/spdlog.h"
#include "cpptrace/cpptrace.hpp"
#include "asio/experimental/awaitable_operators.hpp"

namespace cfgo
{
    namespace gst
    {
        CfgoSrcSPtr::CfgoSrcSPtr(CfgoSrc * pt): PT(pt) {
            spdlog::debug("CfgoSrcSPtr create with {:p}.", fmt::ptr(pt));
        };
        CfgoSrcSPtr::CfgoSrcSPtr(const CfgoSrcSPtr & other): PT(other) {
            spdlog::debug("CfgoSrcSPtr copy.");
        }
        CfgoSrcSPtr::CfgoSrcSPtr(CfgoSrcSPtr && other): PT(std::move(other)) {
            spdlog::debug("CfgoSrcSPtr move.");
        }
        CfgoSrcSPtr::~CfgoSrcSPtr() {
            spdlog::debug("CfgoSrcSPtr destroy with {:p}.", fmt::ptr(get()));
        };
        CfgoSrcSPtr & CfgoSrcSPtr::operator=(const CfgoSrcSPtr & other)
        {
            spdlog::debug("CfgoSrcSPtr copy assign.");
            return static_cast<CfgoSrcSPtr &>(PT::operator=(other));
        }
        CfgoSrcSPtr & CfgoSrcSPtr::operator=(CfgoSrcSPtr && other)
        {
            spdlog::debug("CfgoSrcSPtr move assign.");
            return static_cast<CfgoSrcSPtr &>(PT::operator=(std::move(other)));
        }

        CfgoSrc::Channel::~Channel()
        {
            if (m_pad)
            {
                gst_object_unref(m_pad);
            }
            if (m_processor)
            {
                gst_object_unref(m_processor);
            }
            if (!m_pads.empty())
            {
                spdlog::warn("m_pads should be empty when destructed.");
            }
            for (auto && pad : m_pads)
            {
                gst_object_unref(pad);
            }
            m_pads.clear();
        }

        bool CfgoSrc::Channel::match(GstPad * pad) const
        {
            if (m_pad == pad)
            {
                return true;
            }
            return std::find(m_pads.begin(), m_pads.end(), pad) != m_pads.end();
        }

        void CfgoSrc::Channel::install_ghost(GstCfgoSrcMode m_mode, GstCfgoSrc * owner, GstPad * pad, const std::string & ghost_name)
        {
            auto kclass = GST_ELEMENT_GET_CLASS(owner);
            GstPadTemplate * templ = nullptr;
            if (m_mode == GST_CFGO_SRC_MODE_RAW && ghost_name.starts_with("rtp_src_"))
            {
                templ = gst_element_class_get_pad_template(kclass, "rtp_src_%u_%u_%u");
            }
            else if (m_mode == GST_CFGO_SRC_MODE_PARSE && ghost_name.starts_with("parse_src_"))
            {
                templ = gst_element_class_get_pad_template(kclass, "parse_src_%u_%u_%u");
                gst_object_ref(pad);
                m_pads.push_back(pad);
            }
            else if (m_mode == GST_CFGO_SRC_MODE_DECODE && ghost_name.starts_with("decode_src_"))
            {
                templ = gst_element_class_get_pad_template(kclass, "decode_src_%u_%u_%u");
                gst_object_ref(pad);
                m_pads.push_back(pad);
            }
            auto gpad = gst_ghost_pad_new_from_template(ghost_name.c_str(), pad, templ);
            g_object_set_data(G_OBJECT (pad), "GstCfgoSrc.ghostpad", gpad);
            gst_pad_set_active(gpad, TRUE);
            gst_pad_sticky_events_foreach(pad, copy_sticky_events, gpad);
            gst_element_add_pad(GST_ELEMENT(owner), gpad);
        }

        void CfgoSrc::Channel::uninstall_ghost(GstCfgoSrc * owner, GstPad * pad, bool remove) {
            auto gpad = g_object_get_data(G_OBJECT (pad), "GstCfgoSrc.ghostpad");
            if (gpad)
            {
                gst_pad_set_active(GST_PAD(gpad), FALSE);
                gst_element_remove_pad(GST_ELEMENT(owner), GST_PAD(gpad));
                g_object_set_data(G_OBJECT (pad), "GstCfgoSrc.ghostpad", nullptr);
            }
            if (remove)
            {
                auto iter = std::begin(m_pads);
                while (iter != std::end(m_pads))
                {
                    if (*iter == pad)
                    {
                        iter = m_pads.erase(iter);
                        gst_object_unref(pad);
                    }
                    else
                    {
                        ++iter;
                    }
                }
            }
        }

        auto CfgoSrc::Session::create_channel(CfgoSrc * parent, GstCfgoSrc * owner, guint ssrc, guint pt, GstPad * pad) -> Channel &
        {
            gst_object_ref(pad);
            m_channels.push_back(
                Channel{ .m_pad = pad, .m_sessid = m_id, .m_ssrc = ssrc, .m_pt = pt }
            );
            auto & channel = m_channels.back();
            if (parent->m_mode == GST_CFGO_SRC_MODE_PARSE)
            {
                parent->_create_processor(owner, channel, "parsebin");
            }
            else if (parent->m_mode == GST_CFGO_SRC_MODE_DECODE)
            {
                parent->_create_processor(owner, channel, "decode");
            }
            return channel;
        }

        auto CfgoSrc::Session::find_channel(GstPad * pad) -> Channel &
        {
            auto iter = std::find_if(m_channels.begin(), m_channels.end(), [pad](const Channel & c) {
                return c.match(pad);
            });
            if (iter != m_channels.end())
            {
                return *iter;
            }
            throw cpptrace::runtime_error(fmt::format("Unable to find the channel relate to {}", get_pad_full_name(pad)));
        }

        CfgoSrc::CfgoSrc(int client_handle, const char * pattern_json, const char * req_types_str, guint64 sub_timeout, guint64 read_timeout):
            m_state(INITED),
            m_detached(true), 
            m_client(get_client(client_handle)), 
            m_sub_timeout(sub_timeout), 
            m_read_timeout(read_timeout)
        {
            spdlog::trace("cfgosrc created");
            cfgo_pattern_parse(pattern_json, m_pattern);
            cfgo_req_types_parse(req_types_str, m_req_types);
        }

        CfgoSrc::~CfgoSrc()
        {
            spdlog::trace("cfgosrc destructed");
            m_close_ch.close_no_except("destructed");
            _detach();
        }

        auto CfgoSrc::create(int client_handle, const char * pattern_json, const char * req_types_str, guint64 sub_timeout, guint64 read_timeout) -> Ptr
        {
            return Ptr{new CfgoSrc(client_handle, pattern_json, req_types_str, sub_timeout, read_timeout)};
        }

        void CfgoSrc::set_sub_timeout(guint64 timeout)
        {
            std::lock_guard lock(m_mutex);
            m_sub_timeout = timeout;
        }

        void CfgoSrc::set_sub_try(gint32 tries, guint64 delay_init, guint32 delay_step, guint32 delay_level)
        {
            std::lock_guard lock(m_mutex);
            m_sub_try_option.m_tries = tries;
            m_sub_try_option.m_delay_init = delay_init;
            m_sub_try_option.m_delay_step = delay_step;
            m_sub_try_option.m_delay_level = delay_level;
        }

        void CfgoSrc::set_read_timeout(guint64 timeout)
        {
            std::lock_guard lock(m_mutex);
            m_read_timeout = timeout;
        }

        void CfgoSrc::set_read_try(gint32 tries, guint64 delay_init, guint32 delay_step, guint32 delay_level)
        {
            std::lock_guard lock(m_mutex);
            m_read_try_option.m_tries = tries;
            m_read_try_option.m_delay_init = delay_init;
            m_read_try_option.m_delay_step = delay_step;
            m_read_try_option.m_delay_level = delay_level;
        }

        void CfgoSrc::attach(GstCfgoSrc * owner)
        {
            std::lock_guard lock(m_mutex);
            if (!m_detached)
            {
                if (m_owner == owner)
                {
                    return;
                }
                else
                {
                    throw cpptrace::runtime_error("The owner has been attached, reattach a different owner is forbidden.");
                }
            }
            m_detached = false;
            m_owner = owner;
            m_mode = owner->mode;
            _create_rtp_bin(m_owner);
        }

        void CfgoSrc::_detach()
        {
            if (m_detached)
            {
                return;
            }
            spdlog::trace("_detach");
            m_detached = true;
            for (auto && session : m_sessions)
            {
                if (session.m_rtp_pad)
                {
                    gst_element_release_request_pad(GST_ELEMENT(m_owner), session.m_rtp_pad);
                    gst_object_unref(session.m_rtp_pad);       
                }
                if (session.m_rtcp_pad)
                {
                    gst_element_release_request_pad(GST_ELEMENT(m_owner), session.m_rtcp_pad);
                    gst_object_unref(session.m_rtcp_pad);       
                }
            }
            m_sessions.clear();
            if (m_rtpsrc_enough_data)
            {
                rtp_src_remove_callback(m_owner, m_rtpsrc_enough_data);
            }
            if (m_rtpsrc_need_data)
            {
                rtp_src_remove_callback(m_owner, m_rtpsrc_need_data);
            }
            if (m_rtcpsrc_enough_data)
            {
                rtcp_src_remove_callback(m_owner, m_rtcpsrc_enough_data);
            }
            if (m_rtcpsrc_need_data)
            {
                rtcp_src_remove_callback(m_owner, m_rtcpsrc_need_data);
            }
            if (m_rtp_bin)
            {
                if (m_request_pt_map)
                {
                    g_signal_handler_disconnect(m_rtp_bin, m_request_pt_map);
                }
                if (m_pad_added_handler)
                {
                    g_signal_handler_disconnect(m_rtp_bin, m_pad_added_handler);
                }
                if (m_pad_removed_handler)
                {
                    g_signal_handler_disconnect(m_rtp_bin, m_pad_removed_handler);
                }
                gst_bin_remove(GST_BIN(m_owner), m_rtp_bin);
                m_rtp_bin = nullptr;
            }
            m_owner = nullptr;
        }

        static gboolean
        copy_sticky_events (GstPad * pad, GstEvent ** event, gpointer user_data)
        {
            GstPad *gpad = GST_PAD_CAST (user_data);

            GST_DEBUG_OBJECT (gpad, "store sticky event %" GST_PTR_FORMAT, *event);
            gst_pad_store_sticky_event (gpad, *event);

            return TRUE;
        }

        void CfgoSrc::_create_processor(GstCfgoSrc * owner, Channel & channel, const std::string & type)
        {
            auto processor_name = fmt::sprintf("%s_%u_%u_%u", type, channel.m_sessid, channel.m_ssrc, channel.m_pt);
            auto processor = gst_element_factory_make(type.c_str(), processor_name.c_str());
            if (!processor)
            {
                cfgo_error_submit_general(GST_ELEMENT(owner), ("Unable to create the " + type + " element: " + processor_name + ".").c_str(), TRUE, TRUE);
                return;
            }
            g_signal_connect(processor, "pad-added", G_CALLBACK(pad_added_handler), this);
            g_signal_connect(processor, "pad-removed", G_CALLBACK(pad_removed_handler), this);
            auto sink_pad = gst_element_get_static_pad(processor, "sink");
            DEFER({
                g_object_unref(sink_pad);
            });
            if (GST_PAD_LINK_FAILED(gst_pad_link(channel.m_pad, sink_pad)))
            {
                auto msg = "Unable to link" + cfgo::gst::get_pad_full_name(channel.m_pad) + " to " + cfgo::gst::get_pad_full_name(sink_pad) + ".";
                cfgo_error_submit_general(
                    GST_ELEMENT(owner),
                    msg.c_str(), 
                    TRUE, TRUE
                );
                return;
            }
            gst_bin_add(GST_BIN(owner), processor);
            gst_element_sync_state_with_parent(processor);
            gst_object_ref(processor);
            channel.m_processor = processor;
        }

        std::string get_ghost_pad_name(GstPad * pad)
        {
            if (g_str_has_prefix(GST_PAD_NAME(pad), "recv_rtp_src_"))
            {
                return std::string(GST_PAD_NAME(pad)).substr(5);
            }
            else if (g_str_has_prefix(GST_PAD_NAME(pad), "src_"))
            {
                guint pad_id;
                if (sscanf(GST_PAD_NAME(pad), "src_%u", &pad_id) == 1)
                {
                    auto pad_owner = gst_pad_get_parent_element(pad);
                    if (!pad_owner)
                    {
                        return "";
                    }
                    if (g_str_has_prefix(GST_ELEMENT_NAME(pad_owner), "parsebin_"))
                    {
                        guint sessid, ssrc, pt;
                        if (sscanf(GST_ELEMENT_NAME(pad_owner), "parsebin_%u_%u_%u", &sessid, &ssrc, &pt) == 3)
                        {
                            auto _name = g_strdup_printf("parse_src_%u_%u_%u_%u", sessid, ssrc, pt, pad_id);
                            auto result = std::string(_name);
                            g_free(_name);
                            return result;
                        }
                    }
                    else if (g_str_has_prefix(GST_ELEMENT_NAME(pad_owner), "decodebin_"))
                    {
                        guint sessid, ssrc, pt;
                        if (sscanf(GST_ELEMENT_NAME(pad_owner), "decodebin_%u_%u_%u", &sessid, &ssrc, &pt) == 3)
                        {
                            auto _name = g_strdup_printf("decode_src_%u_%u_%u_%u", sessid, ssrc, pt, pad_id);
                            auto result = std::string(_name);
                            g_free(_name);
                            return result;
                        }
                    }
                }
            }
            return "";
        }

        void CfgoSrc::_install_pad(GstPad * pad)
        {
            _safe_use_owner<void>([this, pad](GstCfgoSrc * owner) {
                auto ghost_name = get_ghost_pad_name(pad);
                if (ghost_name.starts_with("rtp_src_"))
                {
                    guint sessid, ssrc, pt;
                    if (sscanf(GST_PAD_NAME(pad), "recv_rtp_src_%u_%u_%u", &sessid, &ssrc, &pt) == 3)
                    {
                        Session & session = m_sessions[sessid];
                        auto & channel = session.create_channel(this, owner, ssrc, pt, pad);
                        channel.install_ghost(m_mode, owner, pad, ghost_name);
                    }
                }
                else if (ghost_name.starts_with("parse_src_"))
                {
                    guint sessid, ssrc, pt, srcid;
                    if (sscanf(ghost_name.c_str(), "parse_src_%u_%u_%u_%u", &sessid, &ssrc, &pt, &srcid) == 4)
                    {
                        Session & session = m_sessions[sessid];
                        auto & channel = session.find_channel(pad);
                        channel.install_ghost(m_mode, owner, pad, ghost_name);
                    }
                }
                else if (ghost_name.starts_with("decode_src_"))
                {
                    guint sessid, ssrc, pt, srcid;
                    if (sscanf(ghost_name.c_str(), "decode_src_%u_%u_%u_%u", &sessid, &ssrc, &pt, &srcid) == 4)
                    {
                        Session & session = m_sessions[sessid];
                        auto & channel = session.find_channel(pad);
                        channel.install_ghost(m_mode, owner, pad, ghost_name);
                    }
                }
            });
        }

        void CfgoSrc::_uninstall_pad(GstPad * pad)
        {
            _safe_use_owner<void>([this, pad](GstCfgoSrc * owner) {
                auto ghost_name = get_ghost_pad_name(pad);
                if (ghost_name.starts_with("rtp_src_"))
                {
                    guint sessid, ssrc, pt;
                    if (sscanf(GST_PAD_NAME(pad), "recv_rtp_src_%u_%u_%u", &sessid, &ssrc, &pt) == 3)
                    {
                        Session & session = m_sessions[sessid];
                        auto & channel = session.find_channel(pad);
                        channel.uninstall_ghost(owner, pad);
                    }
                }
                else if (ghost_name.starts_with("parse_src_"))
                {
                    guint sessid, ssrc, pt, srcid;
                    if (sscanf(ghost_name.c_str(), "parse_src_%u_%u_%u_%u", &sessid, &ssrc, &pt, &srcid) == 4)
                    {
                        Session & session = m_sessions[sessid];
                        auto & channel = session.find_channel(pad);
                        channel.uninstall_ghost(owner, pad);
                    }
                }
                else if (ghost_name.starts_with("decode_src_"))
                {
                    guint sessid, ssrc, pt, srcid;
                    if (sscanf(ghost_name.c_str(), "decode_src_%u_%u_%u_%u", &sessid, &ssrc, &pt, &srcid) == 4)
                    {
                        Session & session = m_sessions[sessid];
                        auto & channel = session.find_channel(pad);
                        channel.uninstall_ghost(owner, pad);
                    }
                }
            });
        }

        void CfgoSrc::detach()
        {
            if (m_detached)
            {
                return;
            }
            std::lock_guard lock(m_mutex);
            _detach();
        }

        void CfgoSrc::start()
        {
            {
                std::lock_guard lock(m_mutex);   
                if (m_state != INITED)
                {
                    return;
                }
                m_state = RUNNING;
            }
            auto weak_self = weak_from_this();
            asio::co_spawn(
                asio::get_associated_executor(m_client->execution_context()),
                fix_async_lambda([self = shared_from_this()]() -> asio::awaitable<void> {
                    try
                    {
                        co_await self->_loop();
                        self->stop();
                        spdlog::debug("Exit loop.");
                    }
                    catch(...)
                    {
                        spdlog::debug("Exit loop because {}", what());
                    }
                }),
                asio::detached
            );
            spdlog::debug("started.");
        }

        void CfgoSrc::pause()
        {
            std::lock_guard lock(m_mutex);  
            if (m_state == RUNNING)
            {
                m_state = PAUSED;
                m_close_ch.stop();
            }
        }

        void CfgoSrc::resume()
        {
            std::lock_guard lock(m_mutex);  
            if (m_state == PAUSED)
            {
                m_state = RUNNING;
                m_close_ch.resume();
            }
        }

        void CfgoSrc::stop()
        {
            std::lock_guard lock(m_mutex); 
            m_state = STOPED;
            m_close_ch.close("the method cfgosrc::stop called.");
        }

        void rtpsrc_need_data(GstElement * appsrc, guint length, CfgoSrc *self)
        {
            spdlog::debug("{} need {} bytes data", GST_ELEMENT_NAME(appsrc), length);
            std::ignore = self->m_rtp_need_data_ch.try_write();
        }

        void rtpsrc_enough_data(GstElement * appsrc, CfgoSrc *self)
        {
            spdlog::debug("{} say data is enough.", GST_ELEMENT_NAME(appsrc));
            std::ignore = self->m_rtp_enough_data_ch.try_write();
        }

        void rtcpsrc_need_data(GstElement * appsrc, guint length, CfgoSrc *self)
        {
            spdlog::debug("{} need {} bytes data", GST_ELEMENT_NAME(appsrc), length);
            std::ignore = self->m_rtcp_need_data_ch.try_write();
        }

        void rtcpsrc_enough_data(GstElement * appsrc, CfgoSrc *self)
        {
            spdlog::debug("{} say data is enough.", GST_ELEMENT_NAME(appsrc));
            std::ignore = self->m_rtcp_enough_data_ch.try_write();
        }

        GstCaps * request_pt_map(GstElement *src, guint session_id, guint pt, CfgoSrc *self)
        {
            spdlog::debug("[session {}] reqiest pt {}", session_id, pt);
            std::lock_guard lock(self->m_mutex);
            auto & session = self->m_sessions[session_id];
            return (GstCaps *) session.m_track->get_gst_caps(pt);
        }

        void pad_added_handler(GstElement *src, GstPad *new_pad, CfgoSrc *self)
        {
            spdlog::debug("[{}] add pad {}", GST_ELEMENT_NAME(src), GST_PAD_NAME(new_pad));
            self->_install_pad(new_pad);
        }

        void pad_removed_handler(GstElement * src, GstPad * pad, CfgoSrc *self)
        {
            spdlog::debug("[{}] pad {} removed.", GST_ELEMENT_NAME(src), GST_PAD_NAME(pad));
            self->_uninstall_pad(pad);
        }

        void CfgoSrc::_create_rtp_bin(GstCfgoSrc * owner)
        {
            spdlog::debug("Creating rtpbin.");
            m_rtp_bin = gst_element_factory_make("rtpbin", "rtpbin");
            if (!m_rtp_bin)
            {
                throw cpptrace::runtime_error("Unable to create rtpbin.");
            }
            m_rtpsrc_enough_data = rtp_src_add_enough_data_callback(owner, G_CALLBACK(rtpsrc_enough_data), this);
            m_rtpsrc_need_data = rtp_src_add_need_data_callback(owner, G_CALLBACK(rtpsrc_need_data), this);
            m_rtcpsrc_enough_data = rtcp_src_add_enough_data_callback(owner, G_CALLBACK(rtcpsrc_enough_data), this);
            m_rtcpsrc_need_data = rtcp_src_add_need_data_callback(owner, G_CALLBACK(rtcpsrc_need_data), this);
            m_request_pt_map = g_signal_connect(m_rtp_bin, "request-pt-map", G_CALLBACK(request_pt_map), this);
            m_pad_added_handler = g_signal_connect(m_rtp_bin, "pad-added", G_CALLBACK(pad_added_handler), this);
            m_pad_removed_handler = g_signal_connect(m_rtp_bin, "pad-removed", G_CALLBACK(pad_removed_handler), this);
            gst_bin_add(GST_BIN(owner), m_rtp_bin);
            gst_element_sync_state_with_parent(m_rtp_bin);
        }

        auto CfgoSrc::_create_session(GstCfgoSrc * owner, TrackPtr track) -> Session
        {
            auto i = m_sessions.size();
            spdlog::debug("Creating session {}.", i);
            m_sessions.push_back(Session {});
            Session & session = m_sessions.back();
            session.m_id = i;
            session.m_track = track;
            string rtp_pad_name = fmt::sprintf("recv_rtp_sink_%u", i);
            spdlog::debug("Requesting the rtp pad {}.", rtp_pad_name);
            session.m_rtp_pad = gst_element_request_pad_simple(m_rtp_bin, rtp_pad_name.c_str());
            if (!session.m_rtp_pad)
            {
                throw cpptrace::runtime_error(fmt::format("Unable to request the pad {} from rtpbin.", rtp_pad_name));
            }
            link_rtp_src(owner, session.m_rtp_pad);
            string rtcp_pad_name = fmt::sprintf("recv_rtcp_sink_%u", i);
            spdlog::debug("Requesting the rtcp pad {}.", rtcp_pad_name);
            session.m_rtcp_pad = gst_element_request_pad_simple(m_rtp_bin, rtcp_pad_name.c_str());
            if (!session.m_rtcp_pad)
            {
                throw cpptrace::runtime_error(fmt::format("Unable to request the pad {} from rtpbin.", rtcp_pad_name));
            }
            link_rtcp_src(owner, session.m_rtcp_pad);
            spdlog::debug("Session {} created.", i);
            return session;
        }

        auto CfgoSrc::_post_buffer(const Session & session, Track::MsgType msg_type) -> asio::awaitable<void>
        {
            spdlog::debug("Start the {} data task of session {}.", msg_type, session.m_id);
            do
            {
                assert (msg_type != Track::MsgType::ALL);
                if (msg_type == Track::MsgType::RTP)
                {
                    co_await chan_read_or_throw<void>(m_rtp_need_data_ch, m_close_ch);
                }
                else
                {
                    co_await chan_read_or_throw<void>(m_rtcp_need_data_ch, m_close_ch);
                }
                do
                {   
                    TryOption try_option;
                    guint64 read_timeout;
                    {
                        std::lock_guard lock(m_mutex);
                        try_option = m_read_try_option;
                        read_timeout = m_read_timeout;
                    }
                    auto self = shared_from_this();
                    auto track = session.m_track;
                    auto read_task = [self, track, msg_type](auto try_times, auto timeout_closer) -> asio::awaitable<Track::MsgPtr>
                    {
                        if (try_times > 1)
                        {
                            spdlog::debug("Read {} data timeout after {} ms. Tring the {} time.", msg_type, std::chrono::duration_cast<std::chrono::milliseconds>(timeout_closer.get_timeout()), Nth{try_times});
                        }
                        Track::MsgPtr msg_ptr = std::move(co_await track->await_msg(msg_type, timeout_closer));
                        co_return msg_ptr;
                    };
                    auto msg_ptr = co_await async_retry<Track::MsgPtr>(
                        std::chrono::milliseconds {read_timeout},
                        try_option,
                        read_task,
                        [](const Track::MsgPtr & msg) -> bool {
                            return !msg;
                        },
                        m_close_ch
                    );
                    if (!msg_ptr || m_close_ch.is_closed())
                    {
                        if (!m_close_ch.is_closed())
                        {
                            auto error = steal_shared_g_error(create_gerror_timeout(
                                fmt::format("Timeout to read subsrcibed track {} data", (msg_type == Track::MsgType::RTP ? "rtp" : "rtcp"))
                            ));
                            _safe_use_owner<void>([error](auto owner) {
                                cfgo_error_submit(GST_ELEMENT(owner), error.get());
                            });
                        }
                        co_return;
                    }
                    auto msg = std::move(msg_ptr.value());
                    if (!msg)
                    {
                        spdlog::debug("It seems that the track is closed.");
                        co_return;
                    }
                    
                    spdlog::trace("Received {} bytes {} data.", msg->size(), msg_type);
                    auto buffer = _safe_use_owner<GstBuffer *>([&msg](auto owner) {
                        GstBuffer *buffer;
                        buffer = gst_buffer_new_and_alloc(msg->size());
                        auto clock = gst_element_get_clock(GST_ELEMENT(owner));
                        if (!clock)
                        {
                            clock = gst_system_clock_obtain();
                        }
                        DEFER({
                            gst_object_unref(clock);
                        });
                        auto time_now = gst_clock_get_time(clock);
                        auto runing_time = time_now - gst_element_get_base_time(GST_ELEMENT(owner));
                        GST_BUFFER_PTS(buffer) = GST_BUFFER_DTS(buffer) = runing_time;
                        return buffer;
                    });
                    if (!buffer)
                    {
                        co_return;
                    }
                    GstMapInfo info = GST_MAP_INFO_INIT;
                    if (!gst_buffer_map(buffer.value(), &info, GST_MAP_READWRITE))
                    {
                        auto error = steal_shared_g_error(create_gerror_general("Unable to map the buffer", true));
                        _safe_use_owner<void>([error](auto owner) {
                            cfgo_error_submit(GST_ELEMENT(owner), error.get());
                        });
                        co_return;
                    }
                    DEFER({
                        gst_buffer_unmap(buffer.value(), &info);
                    });
                    memcpy(info.data, msg->data(), msg->size());
                    if (!_safe_use_owner<void>([msg_type, buffer = buffer.value()](auto owner) {
                        spdlog::trace("Push {} bytes {} buffer.", gst_buffer_get_size(buffer), msg_type);
                        if (msg_type == Track::MsgType::RTP)
                        {
                            push_rtp_buffer(owner, buffer);
                        }
                        else if (msg_type == Track::MsgType::RTCP)
                        {
                            push_rtcp_buffer(owner, buffer);
                        }
                    }))
                    {
                        co_return;
                    }
                    if (msg_type == Track::MsgType::RTP)
                    {
                        if (m_rtp_enough_data_ch.try_read())
                        {
                            break;
                        }
                    }
                    else
                    {
                        if (m_rtcp_enough_data_ch.try_read())
                        {
                            break;
                        }
                    }
                } while (true);
            } while (true);
        }

        auto CfgoSrc::_loop() -> asio::awaitable<void>
        {
            try
            {
                spdlog::debug("Subscribing...");
                TryOption sub_try_option;
                guint64 sub_timeout;
                {
                    std::lock_guard lock(m_mutex);
                    sub_timeout = m_sub_timeout;
                    sub_try_option = m_sub_try_option;
                }
                auto self = shared_from_this();
                auto sub_task = [self](auto try_times, auto timeout_closer) -> asio::awaitable<SubPtr>
                {
                    if (try_times > 1)
                    {
                        spdlog::debug("Subscribing timeout after {}. Tring the {} time.", timeout_closer.get_timeout(), Nth{try_times});
                    }
                    spdlog::trace("Arg pattern: {}", self->m_pattern);
                    spdlog::trace("Arg req_types: {}", self->m_req_types);
                    auto sub = co_await self->m_client->subscribe(self->m_pattern, self->m_req_types, timeout_closer);
                    co_return sub;
                };
                auto sub = co_await async_retry<SubPtr>(
                    std::chrono::milliseconds {sub_timeout},
                    sub_try_option, 
                    sub_task,
                    [](const SubPtr & sub) -> bool {
                        return !sub;
                    },
                    m_close_ch
                );
                if (!sub)
                {
                    if (!m_close_ch.is_closed())
                    {
                        spdlog::debug("Subscribed timeout.");
                        _safe_use_owner<void>([](auto owner) {
                            auto error = steal_shared_g_error(create_gerror_timeout("Timeout to subscribing."));
                            cfgo_error_submit(GST_ELEMENT(owner), error.get());
                        });
                    }
                    co_return;
                }
                spdlog::debug("Subscribed with {} tracks.", sub.value()->tracks().size());
                cfgo::AsyncTasksAll<void> tasks(m_close_ch);
                for (auto &track : sub.value()->tracks())
                {
                    auto session = _safe_use_owner<Session>([this, track](auto owner) {
                        return _create_session(owner, track);
                    });
                    if (!session)
                    {
                        co_return;
                    }
                    tasks.add_task(fix_async_lambda([self = shared_from_this(), session = session.value()](close_chan closer) -> asio::awaitable<void> {
                        co_await self->_post_buffer(session, Track::MsgType::RTP);
                    }));
                    tasks.add_task(fix_async_lambda([self = shared_from_this(), session = session.value()](close_chan closer) -> asio::awaitable<void> {
                        co_await self->_post_buffer(session, Track::MsgType::RTCP);
                    }));
                }
                try
                {
                    co_await tasks.await();
                }
                catch(const cfgo::CancelError& e)
                {
                    spdlog::debug(fmt::format("The loop task is canceled because {}", e.what()));
                }
            }
            catch(...)
            {
                _safe_use_owner<void>([](auto owner) {
                    auto error = steal_shared_g_error(create_gerror_from_except(std::current_exception(), true));
                    cfgo_error_submit(GST_ELEMENT(owner), error.get());
                });
            }
            co_return;
        }
    } // namespace gst
}
