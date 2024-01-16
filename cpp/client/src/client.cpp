#include "cfgo/client.hpp"
#include "boost/lexical_cast.hpp"
#include "boost/uuid/uuid_generators.hpp"

namespace cfgo {

    Client::Client(const Configuration& config):
    m_config(config),
    m_client(),
    m_id(boost::lexical_cast<std::string>(boost::uuids::random_generator()()))
    {}

    Client::~Client()
    {
        m_client.sync_close();
    }

    void Client::connect() {
        auto auth_msg = sio::object_message::create();
        auth_msg->get_map()["token"] = sio::string_message::create(m_config.m_token);
        auth_msg->get_map()["id"] = sio::string_message::create(m_id);

        m_client.connect(m_config.m_signal_url, auth_msg);
    }
}
