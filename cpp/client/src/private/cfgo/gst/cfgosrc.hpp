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
        class CfgoSrc
        {
        private:
            Client::Ptr m_client;
            Pattern m_pattern;
            std::vector<std::string> m_req_types;
            close_chan m_close_ch;
            GstPad * m_rtp_pad = nullptr;
            GstPad * m_rtcp_pad = nullptr;
            bool m_ready;
            asiochan::channel<void, 1> m_ready_ch;
            guint64 m_sub_timeout;
            guint32 m_sub_tries;
            guint64 m_sub_try_delay_init;
            guint32 m_sub_try_delay_step;
            guint32 m_sub_try_delay_level;
            close_chan m_sub_closer;
            guint64 m_read_timeout;
            guint32 m_read_tries;
            guint64 m_read_try_delay_init;
            guint64 m_read_try_delay_step;
            guint32 m_read_try_delay_level;
            close_chan m_read_closer;
            std::mutex m_mutex;

            void _reset_sub_closer();
            void _reset_read_closer();
            auto _loop(GstCfgoSrc * owner) -> asio::awaitable<void>;
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
            void set_rtp_pad(GstPad * pad);
            void set_rtcp_pad(GstPad * pad);
            void set_sub_timeout(guint64 timeout);
            void set_sub_try(guint32 tries, guint64 delay_init = 0, guint32 delay_step = 0, guint32 delay_level = 0);
            void set_read_timeout(guint64 timeout);
            void set_read_try(guint32 tries, guint64 delay_init = 0, guint32 delay_step = 0, guint32 delay_level = 0);
        };
    } // namespace gst    
} // namespace cfgo


#endif