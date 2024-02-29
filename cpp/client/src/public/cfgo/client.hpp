#ifndef _CFGO_CLIENT_HPP_
#define _CFGO_CLIENT_HPP_

#include "cfgo/alias.hpp"
#include "cfgo/configuration.hpp"
#include "cfgo/pattern.hpp"
#include "cfgo/utils.hpp"
#include "asio.hpp"
#include "rtc/rtc.hpp"
#include "asiochan/asiochan.hpp"
namespace cfgo {
    namespace impl {
        struct Client;
    }

    class Client : ImplBy<impl::Client>
    {
    public:
        using Ptr = std::shared_ptr<Client>;
        using CtxPtr = std::shared_ptr<asio::io_context>;

    public:
        Client(const Configuration& config);
        Client(const Configuration& config, const CtxPtr& io_ctx);
        ~Client();
        [[nodiscard]] asio::awaitable<SubPtr> subscribe(const Pattern& pattern, const std::vector<std::string>& req_types, close_chan& close_chan);
    };
}

#endif