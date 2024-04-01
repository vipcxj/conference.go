#ifndef _GST_CFGO_GST_CFGO_SRC_H_
#define _GST_CFGO_GST_CFGO_SRC_H_

#include "cfgo/client.hpp"
#include "cfgo/track.hpp"
#include "cfgo/async.hpp"
#include "cfgo/spd_helper.hpp"
#include "gst/gst.h"

typedef struct _GstCfgoSrc GstCfgoSrc;

namespace cfgo
{
    namespace gst
    {
        class CfgoSrc;
        class CfgoSrcSPtr : public std::shared_ptr<CfgoSrc>
        {
            using PT = std::shared_ptr<CfgoSrc>;
        public:
            CfgoSrcSPtr(CfgoSrc * pt = nullptr);
            virtual ~CfgoSrcSPtr();
        };

        class CfgoSrc : public std::enable_shared_from_this<CfgoSrc>
        {
        public:
            struct Session
            {
                guint m_id;
                TrackPtr m_track;
                GstPad * m_rtp_pad;
                GstPad * m_rtcp_pad;
            };
            enum State
            {
                INITED,
                RUNNING,
                PAUSED,
                STOPED
            };
        private:
            State m_state;
            Client::Ptr m_client;
            Pattern m_pattern;
            std::vector<std::string> m_req_types;
            close_chan m_close_ch;
            guint64 m_sub_timeout;
            TryOption m_sub_try_option;
            close_chan m_sub_closer;
            guint64 m_read_timeout;
            TryOption m_read_try_option;
            close_chan m_read_closer;
            std::mutex m_mutex;
            std::mutex m_loop_mutex;
            GstCfgoSrc * m_owner;
            bool m_detached;
            GstElement * m_rtp_bin;
            gulong m_request_pt_map = 0;
            gulong m_pad_added_handler = 0;
            gulong m_pad_removed_handler = 0;
            std::vector<Session> m_sessions;

            void _reset_sub_closer();
            void _reset_read_closer();
            void _create_rtp_bin(GstCfgoSrc * owner);
            Session _create_session(GstCfgoSrc * owner, TrackPtr track);
            auto _loop() -> asio::awaitable<void>;
            auto _post_buffer(const Session & session, Track::MsgType msg_type) -> asio::awaitable<void>;
            void _detach();
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
                if constexpr (std::is_void_v<T>)
                {
                    func(m_owner);
                    return make_resolved();
                }
                else
                {
                    return make_resolved<T>(func(m_owner));
                }
            }


        protected:
            CfgoSrc(int client_handle, const char * pattern_json, const char * req_types_str, guint64 sub_timeout, guint64 read_timeout);
            CfgoSrc(const CfgoSrc &) = delete;
            CfgoSrc(CfgoSrc &&) = delete;
            CfgoSrc & operator= (const CfgoSrc &) = delete;
            CfgoSrc & operator= (CfgoSrc &&) = delete;
        public:
            ~CfgoSrc();

            using UPtr = std::unique_ptr<CfgoSrc>;
            using Ptr = CfgoSrcSPtr;
            static auto create(
                int client_handle, 
                const char * pattern_json, 
                const char * req_types_str, 
                guint64 sub_timeout = 0, 
                guint64 read_timeout = 0
            ) -> Ptr;
            void set_sub_timeout(guint64 timeout);
            void set_sub_try(gint32 tries, guint64 delay_init = 0, guint32 delay_step = 0, guint32 delay_level = 0);
            void set_read_timeout(guint64 timeout);
            void set_read_try(gint32 tries, guint64 delay_init = 0, guint32 delay_step = 0, guint32 delay_level = 0);
            void attach(GstCfgoSrc * owner);
            void detach();
            void start();
            void pause();
            void resume();
            void stop();

            friend GstCaps * request_pt_map(GstElement *src, guint session_id, guint pt, CfgoSrc *self);
            friend void pad_added_handler(GstElement *src, GstPad *new_pad, CfgoSrc *self);
            friend void pad_removed_handler(GstElement * src, GstPad * pad, CfgoSrc *self);
        };
    } // namespace gst    
} // namespace cfgo


#endif