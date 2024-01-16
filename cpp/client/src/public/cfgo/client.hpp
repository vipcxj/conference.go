#ifndef _CFGO_CLIENT_HPP_
#define _CFGO_CLIENT_HPP_

#include "cfgo/configuration.hpp"
#include "sio_client.h"
namespace cfgo {

    class Client : std::enable_shared_from_this<Client>
    {
    private:
        Configuration m_config;
        sio::client m_client;
        const std::string m_id;
    public:
        typedef std::shared_ptr<Client> Ptr;
        Client(const Configuration& config);
        ~Client();
        void connect();
    private:
        void bind_evt();
    };
    
    class ClientReady {
    private:
        Client::Ptr m_client;
    public:
        void publish();
    };
}

#endif