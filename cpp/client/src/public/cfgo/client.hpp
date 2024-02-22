#ifndef _CFGO_CLIENT_HPP_
#define _CFGO_CLIENT_HPP_

#include "cfgo/configuration.hpp"
#include "sio_client.h"
#include "asio.hpp"
#include <mutex>
namespace cfgo {
    class Client : std::enable_shared_from_this<Client>
    {
    private:
        Configuration m_config;
        std::unique_ptr<sio::client> m_client;
        const std::string m_id;
    public:
        enum connect_state {
            unknown = 0,
            disconnected = 1,
            connecting = 2,
            connected = 4
        };
        using Ptr = std::shared_ptr<Client>;
        using CtxPtr = std::shared_ptr<asio::io_context>;
        Client() = delete;
        Client(const Configuration& config);
        Client(const Configuration& config, const CtxPtr& io_ctx);
        Client(Client&&) = default;
        ~Client();
        Client(const Client&) = delete;
        Client& operator = (Client&) = delete;
    private:
        CtxPtr m_io_context;
        const bool m_thread_safe;
        std::mutex m_mutex;
        connect_state m_connected;
        CoEvent::Ptr m_connect_state_evt;

        Client(const Configuration& config, const CtxPtr& io_ctx, bool thread_safe);
        void lock();
        void unlock() noexcept;

        asio::awaitable<bool> makesure_connect_(std::chrono::nanoseconds timeout);
        void bind_evt();
        void switch_connect_state(connect_state to, connect_state from = connect_state::unknown);
    public:
        struct Guard {
            Client* const m_client;
            Guard(Client* const client): m_client(client) {
                m_client->lock();
            }
            ~Guard() {
                m_client->unlock();
            }
        };
    };
    
    class ClientReady {
    private:
        Client::Ptr m_client;
    public:
        void publish();
    };
}

#endif