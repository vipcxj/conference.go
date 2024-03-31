#include "cfgo/gst/cfgosrc.hpp"
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
        CfgoSrc::CfgoSrc(int client_handle, const char * pattern_json, const char * req_types_str, guint64 sub_timeout, guint64 read_timeout):
            m_state(INITED),
            m_detached(true), 
            m_client(get_client(client_handle)), 
            m_sub_timeout(sub_timeout), 
            m_read_timeout(m_read_timeout)
        {
            cfgo_pattern_parse(pattern_json, m_pattern);
            cfgo_req_types_parse(req_types_str, m_req_types);
        }

        CfgoSrc::~CfgoSrc()
        {
            m_close_ch.close_no_except();
            _detach();
        }

        void CfgoSrc::_reset_sub_closer()
        {
            m_sub_closer = m_close_ch.create_child();
            m_sub_closer.set_timeout(std::chrono::milliseconds {m_sub_timeout});
        }

        void CfgoSrc::_reset_read_closer()
        {
            m_read_closer = m_close_ch.create_child();
            m_read_closer.set_timeout(std::chrono::milliseconds {m_read_timeout});
        }

        auto CfgoSrc::create(int client_handle, const char * pattern_json, const char * req_types_str, guint64 sub_timeout, guint64 read_timeout) -> UPtr
        {
            return UPtr{new CfgoSrc(client_handle, pattern_json, req_types_str, sub_timeout, read_timeout)};
        }

        void CfgoSrc::set_sub_timeout(guint64 timeout)
        {
            std::lock_guard lock(m_mutex);
            m_sub_timeout = timeout;
            m_sub_closer.set_timeout(std::chrono::milliseconds {m_sub_timeout});
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
            m_read_closer.set_timeout(std::chrono::milliseconds {m_read_timeout});
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
            _create_rtp_bin(m_owner);
        }

        void CfgoSrc::_detach()
        {
            if (m_detached)
            {
                return;
            }
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
            }
            m_owner = nullptr;
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
            std::lock_guard lock(m_mutex);   
            if (m_state != INITED)
            {
                return;
            }
            m_state = RUNNING;
            asio::co_spawn(
                asio::get_associated_executor(m_client->execution_context()),
                [weak_self = weak_from_this()]() -> asio::awaitable<void> {
                    if (auto self = weak_self.lock())
                    {
                        co_await self->_loop();
                        self->stop();
                        spdlog::debug("Exit loop.");
                    }
                },
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
            m_close_ch.close();
        }

        GstCaps * request_pt_map(GstElement *src, guint session_id, guint pt, CfgoSrc *self)
        {
            spdlog::debug("[session {}] reqiest pt {}", session_id, pt);
            auto & session = self->m_sessions[session_id];
            return (GstCaps *) session.m_track->get_gst_caps(pt);
        }

        void pad_added_handler(GstElement *src, GstPad *new_pad, CfgoSrc *self)
        {
            spdlog::debug("[{}] add pad {}", GST_ELEMENT_NAME(src), GST_PAD_NAME(new_pad));
        }

        void pad_removed_handler(GstElement * src, GstPad * pad, CfgoSrc *self)
        {
            spdlog::debug("[{}] pad {} removed.", GST_ELEMENT_NAME(src), GST_PAD_NAME(pad));
        }

        void CfgoSrc::_create_rtp_bin(GstCfgoSrc * owner)
        {
            spdlog::debug("Creating rtpbin.");
            m_rtp_bin = gst_element_factory_make("rtpbin", "rtpbin");
            if (!m_rtp_bin)
            {
                throw cpptrace::runtime_error("Unable to create rtpbin.");
            }
            m_request_pt_map = g_signal_connect(m_rtp_bin, "request-pt-map", G_CALLBACK(request_pt_map), this);
            m_pad_added_handler = g_signal_connect(m_rtp_bin, "pad-added", G_CALLBACK(pad_added_handler), this);
            m_pad_removed_handler = g_signal_connect(m_rtp_bin, "pad-removed", G_CALLBACK(pad_removed_handler), this);
            gst_bin_add(GST_BIN(owner), m_rtp_bin);
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
            string rtcp_pad_name = fmt::sprintf("recv_rtcp_sink_%u", i);
            spdlog::debug("Requesting the rtcp pad {}.", rtcp_pad_name);
            session.m_rtcp_pad = gst_element_request_pad_simple(m_rtp_bin, rtcp_pad_name.c_str());
            if (!session.m_rtcp_pad)
            {
                throw cpptrace::runtime_error(fmt::format("Unable to request the pad {} from rtpbin.", rtcp_pad_name));
            }
            spdlog::debug("Session {} created.", i);
            return session;
        }

        auto CfgoSrc::_post_buffer(const Session & session, Track::MsgType msg_type) -> asio::awaitable<void>
        {
            spdlog::debug("Start the {} data task of session {}.", msg_type, session.m_id);
            do
            {
                TryOption try_option;
                {
                    std::lock_guard lock(m_loop_mutex);
                    try_option = m_read_try_option;
                }
                auto msg_ptr = co_await async_retry<Track::MsgSharedPtr>(
                    try_option,
                    [this, track = session.m_track, msg_type](auto try_times) -> asio::awaitable<Track::MsgSharedPtr> {
                        {
                            std::lock_guard lock(m_loop_mutex);
                            _reset_read_closer();
                        }
                        if (try_times > 1)
                        {
                            spdlog::debug("Read {} data timeout after {}. Tring the {} time.", msg_type, m_sub_closer.get_timeout(), Nth{try_times});
                        }
                        Track::MsgSharedPtr msg_ptr = co_await track->await_msg(msg_type, m_read_closer);
                        co_return msg_ptr;
                    },
                    [this](const Track::MsgSharedPtr & msg) -> bool {
                        return !msg && m_read_closer.is_timeout();
                    },
                    m_close_ch
                );
                if (!msg_ptr)
                {
                    if (m_close_ch.is_timeout() || m_read_closer.is_timeout())
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

                spdlog::trace("Received {} bytes {} data.", msg_ptr.value()->size(), msg_type);
                auto buffer = _safe_use_owner<GstBuffer *>([&](auto owner) {
                    GstBuffer *buffer;
                    buffer = gst_buffer_new_and_alloc(msg_ptr.value()->size());
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
                memcpy(info.data, msg_ptr.value()->data(), msg_ptr.value()->size());
                if (!_safe_use_owner<void>([&session, msg_type, buffer = buffer.value()](auto owner) {
                    spdlog::trace("Send {} bytes {} buffer to pad {}.", gst_buffer_get_size(buffer), msg_type, GST_PAD_NAME(session.m_rtp_pad));
                    if (msg_type == Track::MsgType::RTP && session.m_rtp_pad)
                    {
                        gst_pad_push(session.m_rtp_pad, buffer);
                    }
                    else if (msg_type == Track::MsgType::RTCP && session.m_rtcp_pad)
                    {
                        gst_pad_push(session.m_rtcp_pad, buffer);
                    }
                }))
                {
                    co_return;
                }
            } while (true);
        }

        auto CfgoSrc::_loop() -> asio::awaitable<void>
        {
            try
            {
                spdlog::debug("Subscribing...");
                TryOption sub_try_option;
                {
                    std::lock_guard lock(m_loop_mutex);
                    sub_try_option = m_sub_try_option;
                }
                auto sub = co_await async_retry<SubPtr>(
                    sub_try_option, 
                    [this](auto try_times) -> asio::awaitable<SubPtr> {
                        {
                            std::lock_guard lock(m_loop_mutex);
                            _reset_sub_closer();
                        }
                        if (try_times > 1)
                        {
                            spdlog::debug("Subscribing timeout after {}. Tring the {} time.", m_sub_closer.get_timeout(), Nth{try_times});
                        }
                        spdlog::trace("Arg pattern: {}", m_pattern);
                        spdlog::trace("Arg req_types: {}", m_req_types);
                        auto sub = co_await m_client->subscribe(m_pattern, m_req_types, m_sub_closer);
                        co_return sub;
                    },
                    [this](const SubPtr & sub) -> bool {
                        return !sub && m_sub_closer.is_timeout();
                    },
                    m_close_ch
                );
                if (!sub)
                {
                    if (m_close_ch.is_timeout() || m_sub_closer.is_timeout())
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
                for (auto &&track : sub.value()->tracks())
                {
                    auto session = _safe_use_owner<Session>([this, track](auto owner) {
                        return _create_session(owner, track);
                    });
                    if (!session)
                    {
                        co_return;
                    }
                    tasks.add_task([this, &session](auto closer) -> asio::awaitable<void> {
                        spdlog::debug("rtp task.");
                        co_await _post_buffer(session.value(), Track::MsgType::RTP);
                        co_return;
                    });
                    tasks.add_task([this, &session](auto closer) -> asio::awaitable<void> {
                        spdlog::debug("rtcp task.");
                        co_await _post_buffer(session.value(), Track::MsgType::RTCP);
                        co_return;
                    });
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
