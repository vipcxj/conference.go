#include "cfgo/cfgo.hpp"
#include "cfgo/gst/gst.hpp"

#include "cpptrace/cpptrace.hpp"

#include "Poco/URI.h"
#include "Poco/Net/HTTPClientSession.h"
#include "Poco/Net/HTTPRequest.h"
#include "Poco/Net/HTTPResponse.h"

#include <string>

auto get_token() -> std::string {
    using namespace Poco::Net;
    Poco::URI uri("http://localhost:3100/token?key=10000&uid=10000&uname=user10000&role=parent&room=room0&nonce=12345&autojoin=true");
    HTTPClientSession session(uri.getHost(), uri.getPort());
    HTTPRequest request(HTTPRequest::HTTP_GET, uri.getPathAndQuery(), HTTPMessage::HTTP_1_1);
    HTTPResponse response;
    session.sendRequest(request);
    auto&& rs = session.receiveResponse(response);
    if (response.getStatus() != HTTPResponse::HTTP_OK)
    {
        throw cpptrace::runtime_error("unable to get the token. status code: " + std::to_string((int) response.getStatus()) + ".");
    }
    return std::string{ std::istreambuf_iterator<char>(rs), std::istreambuf_iterator<char>() };
}

auto main_task(cfgo::Client::CtxPtr exec_ctx, const std::string & token) -> asio::awaitable<void> {
    using namespace cfgo;
    cfgo::Configuration conf { "http://localhost:8080", token };
    auto client_ptr = std::make_shared<Client>(conf, exec_ctx);
    Pipeline pipeline("test pipeline", exec_ctx);
    pipeline.add_node("cfgosrc", "cfgosrc");
    g_object_set(
        pipeline.require_node("cfgosrc").get(),
        "client",
        (gint64) cfgo::wrap_client(client_ptr),
        "pattern",
        R"({
            "op": 0,
            "children": [
                {
                    "op": 13,
                    "args": ["video"]
                },
                {
                    "op": 7,
                    "args": ["uid", "0"]
                }
            ]
        })",
        NULL
    );
    pipeline.run();
    co_await pipeline.await();
    co_return;
}

int main(int argc, char **argv) {
    gst_init(&argc, &argv);
    GST_PLUGIN_STATIC_REGISTER(cfgosrc);
    spdlog::set_level(spdlog::level::debug);
    gst_debug_set_threshold_for_name("cfgosrc", GST_LEVEL_TRACE);

    auto pool = std::make_shared<asio::thread_pool>();
    auto token = get_token();
    auto f = asio::co_spawn(asio::get_associated_executor(pool), main_task(pool, token), asio::use_future);
    f.get();
}