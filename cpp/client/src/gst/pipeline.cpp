#include "cfgo/gst/pipeline.hpp"
#include "impl/pipeline.hpp"
#include "cfgo/defer.hpp"
#include "cfgo/utils.hpp"
#include "cpptrace/cpptrace.hpp"

namespace cfgo
{ 
    Pipeline::Pipeline(const std::string & name, CtxPtr exec_ctx): ImplBy(name, exec_ctx) {}

    void Pipeline::run()
    {
        impl()->run();
    }

    void Pipeline::stop()
    {
        impl()->stop();
    }

    auto Pipeline::await(close_chan & close_ch) -> asio::awaitable<bool>
    {
        return impl()->await(close_ch);
    }

    void Pipeline::add_node(const std::string & name, const std::string & type)
    {
        impl()->add_node(name, type);
    }

    GstElementSPtr Pipeline::node(const std::string & name) const
    {
        return make_shared_gst_element(impl()->node(name));
    }

    GstElementSPtr Pipeline::require_node(const std::string & name) const
    {
        return make_shared_gst_element(impl()->require_node(name));
    }

    auto Pipeline::link(const std::string & src, const std::string & target, close_chan & close_ch) -> asio::awaitable<LinkPtr>
    {
        auto link_impl = co_await impl()->link(src, target, close_ch);
        co_return std::shared_ptr<Link>(new Link(link_impl));
    }

    auto Pipeline::link(const std::string & src_name, const std::string & src_pad_name, const std::string & tgt_name, const std::string & tgt_pad_name, close_chan & close_ch) -> asio::awaitable<LinkPtr>
    {
        auto link_impl = co_await impl()->link(src_name, src_pad_name, tgt_name, tgt_pad_name, close_ch);
        co_return std::shared_ptr<Link>(new Link(link_impl));
    }

    const char * Pipeline::name() const noexcept
    {
        return impl()->name();
    }

    const Pipeline::CtxPtr Pipeline::exec_ctx() const noexcept
    {
        return impl()->exec_ctx();
    }

} // namespace cfgo
