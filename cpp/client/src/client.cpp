#include <assert.h>
#include <exception>
#include "cfgo/client.hpp"
#include "impl/client.hpp"

namespace cfgo
{
    Client::Client(const Configuration& config) : ImplBy<impl::Client>(config) {}
    Client::Client(const Configuration& config, const CtxPtr& io_ctx) : ImplBy<impl::Client>(config, io_ctx) {}

    auto Client::subscribe(const Pattern &pattern, const std::vector<std::string> &req_types, close_chan &close_chan) -> asio::awaitable<SubPtr> {
        return impl()->subscribe(pattern, req_types, close_chan);
    }

}
