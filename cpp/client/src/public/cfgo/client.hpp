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
        using Ptr = std::shared_ptr<Client>;
        using CtxPtr = std::shared_ptr<asio::io_context>;
        Client() = delete;
        Client(const Configuration& config);
        Client(const Configuration& config, const CtxPtr& io_ctx);
        Client(const Client&&) = default;
        ~Client();
        Client(const Client&) = delete;
        Client& operator = (Client&) = delete;
    private:
        CtxPtr m_io_context;
        const bool m_thread_safe;
        std::mutex m_mutex;
        bool m_inited;

        Client(const Configuration& config, const CtxPtr& io_ctx, bool thread_safe);
        void lock();
        void unlock();

        asio::awaitable<void> makesure_connect_();
        void bind_evt();
    
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

        inline auto make_guard() {
            return Guard(this);
        }
    };
    
    class ClientReady {
    private:
        Client::Ptr m_client;
    public:
        void publish();
    };
}

#endif