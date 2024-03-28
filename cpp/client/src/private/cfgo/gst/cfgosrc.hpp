#ifndef _GST_CFGO_GST_CFGO_SRC_H_
#define _GST_CFGO_GST_CFGO_SRC_H_

#include "cfgo/client.hpp"
#include "cfgo/async.hpp"
#include "gst/gst.h"

typedef struct _GstCfgoSrc GstCfgoSrc;

namespace cfgo
{
    namespace gst
    {
        class CfgoSrc : public std::enable_shared_from_this<CfgoSrc>
        {   
        private:
            Client::Ptr m_client;
            Pattern m_pattern;
            std::vector<std::string> m_req_types;
            close_chan m_close_ch;
            close_chan m_ready_closer;
            guint64 m_sub_timeout;
            TryOption m_sub_try_option;
            close_chan m_sub_closer;
            guint64 m_read_timeout;
            TryOption m_read_try_option;
            close_chan m_read_closer;
            std::mutex m_mutex;
            std::vector<GstPad *> m_pads;
            GstCfgoSrc * m_owner;
            bool m_detached;

            void _reset_sub_closer();
            void _reset_read_closer();
            auto _loop() -> asio::awaitable<void>;
            auto _post_buffer(GstCfgoSrc * owner, Track::Ptr track, Track::MsgType msg_type) -> asio::awaitable<void>;
            template<typename T>
            cancelable<T> _safe_use_owner(std::function<T(GstCfgoSrc * owner)> func)
            {
                if (m_detached)
                {
                    return make_canceled<T>();
                }
                std::lock_guard lock(m_mutex);
                if (m_detached)
                {
                    return make_canceled<T>();
                }
                return make_resolved<T>(func(m_owner));
            }
        protected:
            CfgoSrc(GstCfgoSrc * owner, int client_handle, const char * pattern_json, const char * req_types_str, guint64 sub_timeout, guint64 read_timeout);
            CfgoSrc(const CfgoSrc &) = delete;
            CfgoSrc(CfgoSrc &&) = delete;
            CfgoSrc & operator= (const CfgoSrc &) = delete;
            CfgoSrc & operator= (CfgoSrc &&) = delete;
        public:
            ~CfgoSrc();

            using UPtr = std::unique_ptr<CfgoSrc>;
            using Ptr = std::shared_ptr<CfgoSrc>;
            static auto create(
                GstCfgoSrc * owner, 
                int client_handle, 
                const char * pattern_json, 
                const char * req_types_str, 
                guint64 sub_timeout = 0, 
                guint64 read_timeout = 0
            ) -> UPtr;
            void set_sub_timeout(guint64 timeout);
            void set_sub_try(gint32 tries, guint64 delay_init = 0, guint32 delay_step = 0, guint32 delay_level = 0);
            void set_read_timeout(guint64 timeout);
            void set_read_try(gint32 tries, guint64 delay_init = 0, guint32 delay_step = 0, guint32 delay_level = 0);
            void detach();
        };
    } // namespace gst    
} // namespace cfgo


#endif