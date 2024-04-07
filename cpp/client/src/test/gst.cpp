#include "cfgo/cfgo.hpp"
#include "cfgo/gst/gst.hpp"

#include "cpptrace/cpptrace.hpp"

#include "Poco/URI.h"
#include "Poco/Net/HTTPClientSession.h"
#include "Poco/Net/HTTPRequest.h"
#include "Poco/Net/HTTPResponse.h"

#include <string>
#include <cstdlib>
#include <thread>
#include <csignal>
#include <set>

volatile std::sig_atomic_t gCtrlCStatus = 0;

void signal_handler(int signal)
{
    gCtrlCStatus = signal;
}

class ControlCHandler
{
private:
    cfgo::close_chan m_closer;
    std::thread m_t;
public:
    ControlCHandler(cfgo::close_chan closer);
    ~ControlCHandler();
};

ControlCHandler::ControlCHandler(cfgo::close_chan closer): m_closer(closer)
{
    m_t = std::thread([closer = m_closer]() mutable {
        while (true)
        {
            if (gCtrlCStatus)
            {
                closer.close();
                break;
            }
            std::this_thread::sleep_for(std::chrono::milliseconds(100));
        }
        spdlog::debug("ctl-c handler completed.");
    });
    m_t.detach();
    std::signal(SIGINT, signal_handler);
}

ControlCHandler::~ControlCHandler()
{}


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

auto main_task(cfgo::Client::CtxPtr exec_ctx, const std::string & token, cfgo::close_chan closer) -> asio::awaitable<void> {
    using namespace cfgo;
    cfgo::Configuration conf { "http://localhost:8080", token };
    auto client_ptr = std::make_shared<Client>(conf, exec_ctx, closer);
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
        "mode",
        GST_CFGO_SRC_MODE_PARSE,
        NULL
    );
    pipeline.add_node("mp4mux", "mp4mux");
    pipeline.add_node("filesink", "filesink");
    g_object_set(
        pipeline.require_node("filesink").get(), 
        "location", 
        (cfgo::getexedir() / "out.mp4").c_str(), 
        NULL
    );
    auto a_link_cfgosrc_mp4mux = pipeline.link_async("cfgosrc", "parse_src_%u_%u_%u_%u", "mp4mux", "video_%u");
    if (!a_link_cfgosrc_mp4mux)
    {
        spdlog::error("Unable to link from cfgosrc to mp4mux");
        co_return;
    }
    auto a_link_mp4mux_fakesink = pipeline.link_async("mp4mux", "src", "filesink", "sink");
    if (!a_link_mp4mux_fakesink)
    {
        spdlog::error("Unable to link from mp4mux to filesink");
        co_return;
    }
    pipeline.run();
    // auto pad_cfgosrc_parse = co_await pipeline.await_pad("cfgosrc", "parse_src_%u_%u_%u_%u", {}, closer);
    // if (!pad_cfgosrc_parse)
    // {
    //     spdlog::error("Unable to got pad parse_src_%u_%u_%u_%u from cfgosrc.");
    //     co_return;
    // }
    // if (auto caps = cfgo::gst::steal_shared_gst_caps(gst_pad_get_allowed_caps(pad_cfgosrc_parse.get())))
    // {
    //     cfgo_gst_print_caps(caps.get(), "");
    //     if (cfgo::gst::caps_check_any(caps.get(), [](auto structure) -> bool {
    //         return g_str_has_prefix(gst_structure_get_name(structure), "video/");
    //     }))
    //     {
    //         spdlog::debug("Found pad {} with video data.", GST_PAD_NAME(pad_cfgosrc_parse.get()));
    //         auto async_link = pipeline.link_async("cfgosrc", "parse_src_%u_%u_%u_%u", "mp4mux", "video_%u");
    //         co_await async_link->await(closer);
    //     }
    //     else if (cfgo::gst::caps_check_any(caps.get(), [](auto structure) -> bool {
    //         return g_str_has_prefix(gst_structure_get_name(structure), "audio/");
    //     }))
    //     {
    //         spdlog::debug("Found pad {} with audio data.", GST_PAD_NAME(pad_cfgosrc_parse.get()));
    //         auto async_link = pipeline.link_async("cfgosrc", "parse_src_%u_%u_%u_%u", "mp4mux", "audio_%u");
    //         co_await async_link->await(closer);
    //     }
    //     else
    //     {
    //         spdlog::error("The pad {} which could not connect to mp4mux.", GST_PAD_NAME(pad_cfgosrc_parse.get()));
    //     }
    // }
    // else
    // {
    //     spdlog::error("Unable to got caps of pad {} from cfgosrc.", GST_PAD_NAME(pad_cfgosrc_parse.get()));
    //     co_return;
    // }
    auto link_cfgosrc_mp4mux = co_await a_link_cfgosrc_mp4mux->await(closer);
    if (!link_cfgosrc_mp4mux)
    {
        spdlog::error("Unable to link from cfgosrc to mp4mux");
        co_return;
    }
    auto link_mp4mux_fakesink = co_await a_link_mp4mux_fakesink->await(closer);
    if (!link_mp4mux_fakesink)
    {
        spdlog::error("Unable to link from mp4mux to fakesink");
        co_return;
    }
    spdlog::debug("linked.");
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
    spdlog::set_level(spdlog::level::debug);
    gst_debug_set_threshold_for_name("cfgosrc", GST_LEVEL_TRACE);

    debug_plugins();
    cfgo::close_chan closer;

    ControlCHandler ctrl_c_handler(closer);

    auto pool = std::make_shared<asio::thread_pool>();
    auto token = get_token();
    GMainLoop *loop = g_main_loop_new(NULL, TRUE);
    auto f = asio::co_spawn(asio::get_associated_executor(pool), cfgo::fix_async_lambda([pool, token, closer]() mutable -> asio::awaitable<void> {
        try
        {
            co_await main_task(pool, token, closer);
        }
        catch(...)
        {
            closer.close();
            spdlog::error(cfgo::what());
        }
    }), asio::use_future);
    g_main_loop_run(loop);
    GST_DEBUG("stopping");
    g_main_loop_unref(loop);
    spdlog::debug("main end");
    return 0;
}