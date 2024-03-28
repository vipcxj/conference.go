#include "cfgo/gst/cfgosrc.hpp"
#include "cfgo/gst/error.hpp"
#include "cfgo/gst/utils.hpp"
#include "cfgo/common.hpp"
#include "cfgo/cfgo.hpp"
#include "cfgo/defer.hpp"
#include "spdlog/spdlog.h"
#include "asio/experimental/awaitable_operators.hpp"

namespace cfgo
{
    namespace gst
    {
        CfgoSrc::CfgoSrc(GstCfgoSrc * owner, int client_handle, const char * pattern_json, const char * req_types_str, guint64 sub_timeout, guint64 read_timeout): 
            m_owner(owner), 
            m_detached(false), 
            m_client(get_client(client_handle)), 
            m_sub_timeout(sub_timeout), 
            m_read_timeout(m_read_timeout)
        {
            cfgo_pattern_parse(pattern_json, m_pattern);
            cfgo_req_types_parse(req_types_str, m_req_types);
            asio::co_spawn(
                asio::get_associated_executor(m_client->execution_context()),
                [weak_self = weak_from_this()]() -> asio::awaitable<void> {
                    if (auto self = weak_self.lock())
                    {
                        co_await self->_loop();
                    }
                },
                asio::detached
            );
        }

        CfgoSrc::~CfgoSrc()
        {
            m_close_ch.close_no_except();
            if (m_rtp_pad)
            {
                gst_object_unref(m_rtp_pad);
            }
            if (m_rtcp_pad)
            {
                gst_object_unref(m_rtcp_pad);
            }
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

        auto CfgoSrc::create(GstCfgoSrc * owner, int client_handle, const char * pattern_json, const char * req_types_str, guint64 sub_timeout, guint64 read_timeout) -> UPtr
        {
            return UPtr{new CfgoSrc(owner, client_handle, pattern_json, req_types_str, sub_timeout, read_timeout)};
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

        void CfgoSrc::detach()
        {
            if (m_detached)
            {
                return;
            }
            std::lock_guard lock(m_mutex);
            m_detached = true;
            m_owner = nullptr;
        }

        auto CfgoSrc::_post_buffer(GstCfgoSrc * owner, Track::Ptr track, Track::MsgType msg_type) -> asio::awaitable<void>
        {
            gst_object_ref(owner);
            DEFER({
                gst_object_unref(owner);
            });
            do
            {
                TryOption try_option;
                {
                    std::lock_guard lock(m_mutex);
                    try_option = m_read_try_option;
                }
                auto msg_ptr = co_await async_retry<Track::MsgPtr>(
                    try_option,
                    [this, track, msg_type]() -> asio::awaitable<Track::MsgPtr> {
                        {
                            std::lock_guard lock(m_mutex);
                            _reset_read_closer();
                        }
                        return track->await_msg(msg_type, m_read_closer);
                    },
                    [this](Track::MsgPtr msg) -> bool {
                        return !msg && m_read_closer.is_timeout();
                    },
                    m_close_ch
                );
                if (!msg_ptr)
                {
                    if (m_close_ch.is_timeout() || m_read_closer.is_timeout())
                    {
                        auto error = steal_shared_g_error(create_gerror_timeout(std::string("Timeout to read subsrcibed track ") + (msg_type == Track::MsgType::RTP ? "rtp" : "rtcp") + " data."));
                        cfgo_error_submit(GST_ELEMENT(owner), error.get());
                    }
                    co_return;
                }
                GstBuffer *buffer;
                buffer = gst_buffer_new_and_alloc(msg_ptr.value()->size());
                auto clock = gst_system_clock_obtain();
                DEFER({
                    gst_object_unref(clock);
                });
                auto time_now = gst_clock_get_time(clock);
                auto runing_time = time_now - gst_element_get_base_time(GST_ELEMENT(owner));
                GST_BUFFER_PTS(buffer) = GST_BUFFER_DTS(buffer) = runing_time;

                GstMapInfo info = GST_MAP_INFO_INIT;
                if (!gst_buffer_map(buffer, &info, GST_MAP_READWRITE))
                {
                    auto error = steal_shared_g_error(create_gerror_general("Unable to map the buffer", true));
                    cfgo_error_submit(GST_ELEMENT(owner), error.get());
                    co_return;
                }
                DEFER({
                    gst_buffer_unmap(buffer, &info);
                });
                memcpy(info.data, msg_ptr.value()->data(), msg_ptr.value()->size());
                if (msg_type == Track::MsgType::RTP)
                {
                    std::lock_guard lock(m_mutex);
                    if (m_rtp_pad)
                    {
                        gst_pad_push(m_rtp_pad, buffer);
                    }
                    else
                    {
                        co_return;
                    }
                }
                else
                {
                    std::lock_guard lock(m_mutex);
                    if (m_rtcp_pad)
                    {
                        gst_pad_push(m_rtcp_pad, buffer);
                    }
                    else
                    {
                        co_return;
                    }
                }
            } while (true);
        }

        auto CfgoSrc::_loop() -> asio::awaitable<void>
        {
            co_await m_ready_closer.await();
            try
            {
                TryOption sub_try_option;
                {
                    std::lock_guard lock(m_mutex);
                    sub_try_option = m_sub_try_option;
                }
                auto sub = co_await async_retry<SubPtr>(
                    sub_try_option, 
                    [this]() -> asio::awaitable<SubPtr> {
                        {
                            std::lock_guard lock(m_mutex);
                            _reset_sub_closer();
                        }
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
                        _safe_use_owner<void>([](auto owner) {
                            auto error = steal_shared_g_error(create_gerror_timeout("Timeout to subscribing."));
                            cfgo_error_submit(GST_ELEMENT(owner), error.get());
                        });
                    }
                    co_return;
                }
                using namespace asio::experimental::awaitable_operators;
                auto tasks = []() -> asio::awaitable<void> { co_return; }();
                for (auto &&track : sub.value()->tracks())
                {
                    tasks = std::move(std::move(tasks)
                    && [this, track]() -> asio::awaitable<void> {
                        _post_buffer(track, Track::MsgType::RTP);
                    }()
                    && [this, track]() -> asio::awaitable<void> {
                        _post_buffer(track, Track::MsgType::RTCP);
                    }());
                }
                co_await std::move(tasks);
            }
            catch(...)
            {
                _safe_use_owner<void>([](auto owner) {
                    auto error = steal_shared_g_error(create_gerror_from_except(std::current_exception(), true));
                    cfgo_error_submit(GST_ELEMENT(owner), error.get());
                });
            }
        }
    } // namespace gst
}
