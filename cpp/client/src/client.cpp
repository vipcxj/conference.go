#include <assert.h>
#include <exception>
#include "cfgo/client.hpp"
#include "impl/client.hpp"

namespace cfgo
{
    Client::Client(const Configuration& config) : ImplBy<impl::Client>(config) {}
    Client::Client(const Configuration& config, const CtxPtr& io_ctx) : ImplBy<impl::Client>(config, io_ctx) {}

    void Client::set_sio_logs_default() {
        impl()->set_sio_logs_default();
    }
    void Client::set_sio_logs_verbose() {
        impl()->set_sio_logs_verbose();
    }
    void Client::set_sio_logs_quiet() {
        impl()->set_sio_logs_quiet();
    }

    auto Client::subscribe(const Pattern &pattern, const std::vector<std::string> &req_types, close_chan &close_chan) -> asio::awaitable<SubPtr> {
        return impl()->subscribe(pattern, req_types, close_chan);
    }

}
