#ifndef _CFGO_CLIENT_IMPL_HPP_
#define _CFGO_CLIENT_IMPL_HPP_

#include "cfgo/alias.hpp"
#include "cfgo/configuration.hpp"
#include "cfgo/pattern.hpp"
#include "cfgo/utils.hpp"
#include "sio_client.h"
#include "asio.hpp"
#include "asiochan/asiochan.hpp"
#include <mutex>
#include <optional>

namespace rtc
{
    class PeerConnection;
} // namespace rtc


namespace cfgo {
    namespace impl {
        class Client : std::enable_shared_from_this<Client>
        {
        public:
            using Ptr = std::shared_ptr<Client>;
            using CtxPtr = std::shared_ptr<asio::io_context>;
            struct Guard {
                Client* const m_client;
                Guard(Client* const client): m_client(client) {
                    m_client->lock();
                }
                ~Guard() {
                    m_client->unlock();
                }
            };

            struct MsgChanner {
                Client* const m_client;
                std::map<std::string, msg_chan> m_chan_map;
                MsgChanner(Client* const client);
                void prepare(const std::string &evt, msg_ptr const& ack = nullptr);
                void release(const std::string &evt);
                msg_chan &chan(const std::string &evt);
                const msg_chan &chan(const std::string &evt) const;
                ~MsgChanner();
            };
        private:
            Configuration m_config;
            std::unique_ptr<sio::client> m_client;
            std::shared_ptr<rtc::PeerConnection> m_peer;
            const std::string m_id;
        public:
            Client() = delete;
            Client(const Configuration& config);
            Client(const Configuration& config, const CtxPtr& io_ctx);
            Client(Client&&) = default;
            ~Client();
            Client(const Client&) = delete;
            Client& operator = (Client&) = delete;
            [[nodiscard]] asio::awaitable<SubPtr> subscribe(const Pattern& pattern, const std::vector<std::string>& req_types, close_chan& close_chan);
        private:
            bool m_busy;
            using busy_chan = asiochan::channel<void>;
            std::vector<busy_chan> m_busy_chans;
            [[nodiscard]] asio::awaitable<bool> accquire(close_chan& close_chan);
            void release();

            [[nodiscard]] msg_ptr create_auth_message() const;

            template<typename T>
            void write_ch(asiochan::channel<T>& ch, const T& v) {
                asio::co_spawn(*m_io_context, ch.write(v), asio::detached);
            };
            void write_ch(asiochan::channel<void>& ch) {
                asio::co_spawn(*m_io_context, ch.write(), asio::detached);
            };
            void emit(const std::string& evt, msg_ptr msg);
            [[nodiscard]] asio::awaitable<std::optional<msg_ptr>> emit_with_ack(const std::string& evt, msg_ptr msg, close_chan& close_chan) const;
            [[nodiscard]] asio::awaitable<std::optional<msg_ptr>> wait_for_msg(const std::string& evt, MsgChanner& msg_channer, close_chan& close_chan, std::function<bool(msg_ptr)> cond);
            [[nodiscard]] asio::awaitable<std::optional<bool>> _unsubscribe(const std::string& sub_id, close_chan& close_chan);
            void add_candidate(const msg_ptr& msg);

            CtxPtr m_io_context;
            const bool m_thread_safe;
            std::mutex m_mutex;

            Client(const Configuration& config, const CtxPtr& io_ctx, bool thread_safe);
            void lock();
            void unlock() noexcept;
        };
    }
}

#endif