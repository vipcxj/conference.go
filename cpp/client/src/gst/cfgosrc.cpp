#include "cfgo/gst/cfgosrc.hpp"
#include "cfgo/gst/error.hpp"
#include "cfgo/gst/utils.hpp"
#include "cfgo/cfgo.hpp"
#include "cfgo/defer.hpp"
#include "spdlog/spdlog.h"

namespace cfgo
{
    namespace gst
    {
        CfgoSrc::CfgoSrc(GstCfgoSrc * owner, int client_handle, const char * pattern_json, const char * req_types_str, guint64 sub_timeout, guint64 read_timeout)
        : m_client(get_client(client_handle)), m_sub_timeout(sub_timeout), m_read_timeout(m_read_timeout), m_ready(false)
        {
            cfgo_pattern_parse(pattern_json, m_pattern);
            cfgo_req_types_parse(req_types_str, m_req_types);
            gst_object_ref(owner);
            asio::co_spawn(asio::get_associated_executor(m_client->execution_context()), _loop(owner), asio::detached);
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

        void CfgoSrc::set_rtp_pad(GstPad * pad)
        {
            std::lock_guard lock(m_mutex);
            if (m_rtp_pad != pad)
            {
                gst_object_ref(pad);
                m_rtp_pad = pad;
                if (!m_ready)
                {
                    if (m_rtp_pad && m_rtcp_pad)
                    {
                        m_ready = true;
                        m_read_closer.close();
                    }
                }
            }
        }

        void CfgoSrc::set_rtcp_pad(GstPad * pad)
        {
            std::lock_guard lock(m_mutex);
            if (m_rtcp_pad != pad)
            {
                gst_object_ref(pad);
                m_rtcp_pad = pad;
                if (!m_ready)
                {
                    if (m_rtp_pad && m_rtcp_pad)
                    {
                        m_ready = true;
                        m_read_closer.close();
                    }
                }
            }
        }

        void CfgoSrc::set_sub_timeout(guint64 timeout)
        {
            std::lock_guard lock(m_mutex);
            m_sub_timeout = timeout;
            m_sub_closer.set_timeout(std::chrono::milliseconds {m_sub_timeout});
        }

        void CfgoSrc::set_sub_try(guint32 tries, guint64 delay_init, guint32 delay_step, guint32 delay_level)
        {
            std::lock_guard lock(m_mutex);
            m_sub_tries = tries;
            m_sub_try_delay_init = delay_init;
            m_sub_try_delay_step = delay_step;
            m_sub_try_delay_level = delay_level;
        }

        void CfgoSrc::set_read_timeout(guint64 timeout)
        {
            std::lock_guard lock(m_mutex);
            m_read_timeout = timeout;
            m_read_closer.set_timeout(std::chrono::milliseconds {m_read_timeout});
        }

        void CfgoSrc::set_read_try(guint32 tries, guint64 delay_init, guint32 delay_step, guint32 delay_level)
        {
            std::lock_guard lock(m_mutex);
            m_read_tries = tries;
            m_read_try_delay_init = delay_init;
            m_read_try_delay_step = delay_step;
            m_read_try_delay_level = delay_level;
        }

        auto CfgoSrc::_loop(GstCfgoSrc * owner) -> asio::awaitable<void>
        {
            DEFER({
                gst_object_unref(owner);
            });
            try
            {
                guint32 sub_tries;
                guint64 sub_try_delay_init;
                guint32 sub_try_delay_step;
                guint32 sub_try_delay_level;
                {
                    std::lock_guard lock(m_mutex);
                    guint32 sub_tries = m_sub_tries;
                    guint64 sub_try_delay_init = m_sub_try_delay_init;
                    guint32 sub_try_delay_step = m_sub_try_delay_step;
                    guint32 sub_try_delay_level = m_sub_try_delay_level;
                }
                do
                {
                    {
                        std::lock_guard lock(m_mutex);
                        _reset_sub_closer();
                    }
                    auto sub = co_await m_client->subscribe(m_pattern, m_req_types, m_sub_closer);

                } while (true);
            }
            catch(...)
            {
                auto error = steal_shared_g_error(create_gerror_from_except(std::current_exception(), true));
                auto message = gst_message_new_error(GST_OBJECT(owner), error.get(), cfgo_error_get_trace(error.get()));
                if (!message || !gst_element_post_message(GST_ELEMENT(owner), message))
                {
                    spdlog::warn("Failed to post the error message to gst element {}. The error is \"{}\".", 
                        GST_ELEMENT_NAME(owner), 
                        cfgo_error_get_message(error.get())
                    );
                }
                if (message)
                {
                    gst_message_unref(message);
                }
            }
        }
    } // namespace gst
}
