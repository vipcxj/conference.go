#include "cfgo/gst/error.h"
#include "cfgo/utils.hpp"
#include <exception>
#include <mutex>
#include <memory>
#include "cpptrace/cpptrace.hpp"

namespace cfgo
{
    namespace gst
    {
        struct ErrorPrivateState
        {
            std::exception_ptr m_except_ptr;
            bool m_cached;
            std::string m_trace_str;
            std::string m_message;
            std::mutex m_mutex;

            void _capture();
            explicit ErrorPrivateState(std::exception_ptr except);
            const char * get_trace_str();
            const char * get_message();
        };

        ErrorPrivateState::ErrorPrivateState(std::exception_ptr except): m_except_ptr(except), m_cached(false)
        {}

        void ErrorPrivateState::_capture()
        {
            m_cached = true;
            if (!m_except_ptr)
            {
                return;
            }
            try
            {
                std::rethrow_exception(m_except_ptr);
            }
            catch(const cpptrace::exception & e)
            {
                m_trace_str = e.trace().to_string();
                m_message = e.message();
                return;
            }
            m_message = cfgo::what(m_except_ptr);
        }

        const char * ErrorPrivateState::get_trace_str()
        {
            if (m_cached)
            {
                return m_trace_str.c_str();
            }
            std::lock_guard lock(m_mutex);
            _capture();
            return m_trace_str.c_str();
        }

        const char * ErrorPrivateState::get_message()
        {
            if (m_cached)
            {
                return m_message.c_str();
            }
            std::lock_guard lock(m_mutex);
            _capture();
            return m_message.c_str();
        }
    } // namespace gst
    
} // namespace cfgo


typedef struct
{
    std::shared_ptr<cfgo::gst::ErrorPrivateState> m_state;
} CfgoErrorPrivate;

static void
cfgo_error_private_init (CfgoErrorPrivate *priv)
{
    priv->m_state = nullptr;
}

static void
cfgo_error_private_copy (const CfgoErrorPrivate * src_priv, CfgoErrorPrivate * dest_priv)
{
    dest_priv->m_state = src_priv->m_state;
}

static void
cfgo_error_private_clear (CfgoErrorPrivate * priv)
{}

// This defines the cfgo_error_get_private and cfgo_error_quark functions.
G_DEFINE_EXTENDED_ERROR (CfgoError, cfgo_error)

const gchar * cfgo_error_get_trace (GError *error)
{
    auto priv = cfgo_error_get_private(error);
    if (!priv || !priv->m_state)
    {
        return "";
    }
    return priv->m_state->get_trace_str();
}

const gchar * cfgo_error_get_message (GError *error)
{
    auto msg = error->message;
    if (msg && strlen(msg) > 0)
    {
        return msg;
    }
    auto priv = cfgo_error_get_private(error);
    if (!priv || !priv->m_state)
    {
        return "";
    }
    return priv->m_state->get_message();
}