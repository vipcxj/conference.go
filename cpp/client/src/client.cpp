#include <assert.h>
#include "cfgo/client.hpp"
#include "cfgo/cothread.hpp"
#include "cfgo/coevent.hpp"
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
    m_connected(connect_state::disconnected),
    m_connect_state_evt(nullptr)
    {
        bind_evt();
    }

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

    void Client::unlock() noexcept {
        if (!m_thread_safe)
        {
            m_mutex.unlock();
        }
    }

    void Client::switch_connect_state(connect_state to, connect_state from) {
        Client::Guard(this);
        if (from != connect_state::unknown)
        {
            assert(m_connected == from);
        }
        m_connected = to;
        if (m_connect_state_evt)
        {
            m_connect_state_evt->trigger();
            m_connect_state_evt = nullptr;
        }
    }

    asio::awaitable<bool> Client::makesure_connect_(std::chrono::nanoseconds timeout) {
        bool success = false;
        CoEvent::Ptr connect_state_evt = nullptr;
        {
            Client::Guard(this);
            if (m_connected == connect_state::connected)
            {
                success = true;
            }
            else if (m_connected == connect_state::connecting)
            {
                if (!m_connect_state_evt)
                {
                    m_connect_state_evt = CoEvent::create();
                }
                connect_state_evt = m_connect_state_evt;
            } else {
                m_connected = connect_state::connecting;
            }
        }
        if (success)
        {
            co_return true;
        }
        if (connect_state_evt)
        {
            co_await connect_state_evt->await(timeout);
            co_return m_connected == connect_state::connected;
        }
        
        auto res_evt = cfgo::CoEvent::create();
        cfgo::CoEvent::WeakPtr weak_res_evt = res_evt;
        auto weak_self = weak_from_this();
        co_await make_void_awaitable([&success, weak_self, weak_res_evt]() {
            if (auto self = weak_self.lock())
            {
                auto auth_msg = sio::object_message::create();
                auth_msg->get_map()["token"] = sio::string_message::create(self->m_config.m_token);
                auth_msg->get_map()["id"] = sio::string_message::create(self->m_id);

                self->m_client->set_socket_open_listener([&success, weak_self, weak_res_evt](std::string const& nsp) mutable {
                    if (auto self = weak_self.lock())
                    {
                        self->switch_connect_state(connect_state::connected, connect_state::connecting);
                    }
                    if (auto res_evt = weak_res_evt.lock())
                    {
                        success = true;
                        res_evt->trigger();
                    }
                });
                self->m_client->set_fail_listener([weak_self, weak_res_evt]() {
                    if (auto self = weak_self.lock())
                    {
                        self->switch_connect_state(connect_state::disconnected, connect_state::connecting);
                    }
                    if (auto res_evt = weak_res_evt.lock())
                    {
                        res_evt->trigger();
                    }
                });
                self->m_client->connect(self->m_config.m_signal_url, auth_msg);
            }
        }, asio::use_awaitable);
        co_await res_evt->await(timeout);
        co_return success;
    }

    void Client::bind_evt() {
        auto weak_self = weak_from_this();
        m_client->set_socket_close_listener([weak_self](std::string const& nsp) {
            if (auto self = weak_self.lock())
            {
                self->switch_connect_state(connect_state::disconnected, connect_state::unknown);
            }
        });
        m_client->socket()->on("subscribed", )
    }
}
