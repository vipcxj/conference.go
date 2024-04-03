#ifndef _GST_CFGO_GST_CFGO_SRC_H_
#define _GST_CFGO_GST_CFGO_SRC_H_

#include "cfgo/client.hpp"
#include "cfgo/track.hpp"
#include "cfgo/async.hpp"
#include "cfgo/spd_helper.hpp"
#include "cfgo/gst/gstcfgosrc.h"
#include "gst/gst.h"
#include <vector>

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
            CfgoSrcSPtr(const CfgoSrcSPtr &);
            CfgoSrcSPtr(CfgoSrcSPtr &&);
            virtual ~CfgoSrcSPtr();
            CfgoSrcSPtr & operator=(const CfgoSrcSPtr &);
            CfgoSrcSPtr & operator=(CfgoSrcSPtr &&);
        };

        class CfgoSrc : public std::enable_shared_from_this<CfgoSrc>
        {
        public:
            struct Channel
            {
                GstPad * m_pad = nullptr;
                guint m_sessid = 0;
                guint m_ssrc = 0;
                guint m_pt = 0;
                GstElement * m_processor = nullptr;
                std::vector<GstPad *> m_pads;

                ~Channel();
                bool match(GstPad * pad) const;
                void install_ghost(GstCfgoSrcMode m_mode, GstCfgoSrc * owner, GstPad * pad, const std::string & ghost_name);
                void uninstall_ghost(GstCfgoSrc * owner, GstPad * pad, bool remove = true);
            };
            struct Session
            {
                guint m_id;
                TrackPtr m_track;
                GstPad * m_rtp_pad;
                GstPad * m_rtcp_pad;
                std::vector<Channel> m_channels;
                Channel & create_channel(CfgoSrc * parent, GstCfgoSrc * owner, guint ssrc, guint pt, GstPad * pad);
                Channel & find_channel(GstPad * pad);
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
            GstCfgoSrcMode m_mode;
            Client::Ptr m_client;
            Pattern m_pattern;
            std::vector<std::string> m_req_types;
            close_chan m_close_ch;
            asiochan::channel<void, 1> m_rtp_need_data_ch;
            asiochan::channel<void, 1> m_rtp_enough_data_ch;
            asiochan::channel<void, 1> m_rtcp_need_data_ch;
            asiochan::channel<void, 1> m_rtcp_enough_data_ch;
            guint64 m_sub_timeout;
            TryOption m_sub_try_option;
            guint64 m_read_timeout;
            TryOption m_read_try_option;
            std::mutex m_mutex;
            GstCfgoSrc * m_owner;
            bool m_detached;
            GstElement * m_rtp_bin;
            gulong m_rtpsrc_need_data = 0;
            gulong m_rtpsrc_enough_data = 0;
            gulong m_rtcpsrc_need_data = 0;
            gulong m_rtcpsrc_enough_data = 0;
            gulong m_request_pt_map = 0;
            gulong m_pad_added_handler = 0;
            gulong m_pad_removed_handler = 0;
            std::vector<Session> m_sessions;

            void _reset_sub_closer();
            void _reset_read_closer();
            void _create_rtp_bin(GstCfgoSrc * owner);
            Session _create_session(GstCfgoSrc * owner, TrackPtr track);
            void _create_processor(GstCfgoSrc * owner, Channel & channel, const std::string & type);
            auto _loop() -> asio::awaitable<void>;
            auto _post_buffer(const Session & session, Track::MsgType msg_type) -> asio::awaitable<void>;
            void _detach();
            void _install_pad(GstPad * pad);
            void _uninstall_pad(GstPad * pad);
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
            using Ptr = std::shared_ptr<CfgoSrc>;
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

            friend void rtpsrc_need_data(GstElement * appsrc, guint length, CfgoSrc *self);
            friend void rtpsrc_enough_data(GstElement * appsrc, CfgoSrc *self);
            friend void rtcpsrc_need_data(GstElement * appsrc, guint length, CfgoSrc *self);
            friend void rtcpsrc_enough_data(GstElement * appsrc, CfgoSrc *self);
            friend GstCaps * request_pt_map(GstElement *src, guint session_id, guint pt, CfgoSrc *self);
            friend void pad_added_handler(GstElement *src, GstPad *new_pad, CfgoSrc *self);
            friend void pad_removed_handler(GstElement * src, GstPad * pad, CfgoSrc *self);
        };
    } // namespace gst    
} // namespace cfgo


#endif