#include "cfgo/cfgo.hpp"
#include "cfgo/gst/gst.hpp"

#include "cpptrace/cpptrace.hpp"

#include "Poco/URI.h"
#include "Poco/Net/HTTPClientSession.h"
#include "Poco/Net/HTTPRequest.h"
#include "Poco/Net/HTTPResponse.h"

#include <string>
#include <cstdlib>

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
    gst::Pipeline pipeline("test pipeline", exec_ctx);
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

void debug_plugins()
{
    auto plugins = gst_registry_get_plugin_list(gst_registry_get());
    DEFER({
        gst_plugin_list_free(plugins);
    });
    g_print("PATH: %s\n", std::getenv("PATH"));
    g_print("GST_PLUGIN_PATH: %s\n", std::getenv("GST_PLUGIN_PATH"));
    int plugins_num = 0;
    while (plugins)
    {
        auto plugin = (GstPlugin *) (plugins->data);
        plugins = g_list_next(plugins);
        g_print("plugin: %s\n", gst_plugin_get_name(plugin));
        ++plugins_num;
    }
    g_print("found %d plugins\n", plugins_num);
}

int main(int argc, char **argv) {
    cpptrace::register_terminate_handler();
    gst_init(&argc, &argv);
    GST_PLUGIN_STATIC_REGISTER(cfgosrc);
    spdlog::set_level(spdlog::level::trace);
    gst_debug_set_threshold_for_name("cfgosrc", GST_LEVEL_TRACE);

    debug_plugins();

    auto pool = std::make_shared<asio::thread_pool>();
    auto token = get_token();
    auto f = asio::co_spawn(asio::get_associated_executor(pool), main_task(pool, token), asio::use_future);
    f.get();
}