#include "cfgo/client.hpp"
#include "cfgo/cothread.hpp"
#include "boost/lexical_cast.hpp"
#include "boost/uuid/uuid_io.hpp"
#include "boost/uuid/uuid_generators.hpp"
#include "asiochan/asiochan.hpp"

namespace cfgo {

    Client::Client(const Configuration& config):
    Client(config, std::make_shared<asio::io_context>(), true)
    {}

    Client::Client(const Configuration& config, const CtxPtr& io_ctx):
    Client(config, io_ctx, config.m_thread_safe)
    {}

    Client::Client(const Configuration& config, const CtxPtr& io_ctx, bool thread_safe):
    m_config(config),
    m_client(),
    m_id(boost::lexical_cast<std::string>(boost::uuids::random_generator()())),
    m_io_context(io_ctx),
    m_thread_safe(thread_safe),
    m_mutex(),
    m_inited(false)
    {}

    Client::~Client()
    {
        m_client->sync_close();
    }

    void Client::lock() {
        if (!m_thread_safe)
        {
            m_mutex.lock();
        }
    }

    void Client::unlock() {
        if (!m_thread_safe)
        {
            m_mutex.unlock();
        }
    }

    asio::awaitable<void> Client::makesure_connect_() {
        if (auto self = weak_from_this().lock())
        {
            bool should_connect = false;
            {
                self->make_guard();
                if (!self->m_inited)
                {
                    self->m_inited = true;
                    should_connect = true;
                }
            }
            if (should_connect)
            {
                co_await make_void_awaitable([self]() {
                    auto auth_msg = sio::object_message::create();
                    auth_msg->get_map()["token"] = sio::string_message::create(self->m_config.m_token);
                    auth_msg->get_map()["id"] = sio::string_message::create(self->m_id);

                    self->m_client->set_open_listener([]() {

                    });

                    self->m_client->connect(self->m_config.m_signal_url, auth_msg);

                }, asio::use_awaitable);
            }
        }
        co_return;
    }
}
