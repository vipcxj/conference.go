#ifndef _GST_CFGO_GST_PIPELINE_H_
#define _GST_CFGO_GST_PIPELINE_H_

#include "cfgo/alias.hpp"
#include "cfgo/async.hpp"
#include "cfgo/utils.hpp"
#include "cfgo/gst/utils.hpp"
#include "cfgo/gst/link.hpp"
#include "gst/gst.h"
#include "asio/awaitable.hpp"

namespace cfgo
{
    namespace gst
    {
        namespace impl
        {
            class Pipeline;
        } // namespace impl
        

        class Pipeline : public ImplBy<impl::Pipeline>
        {
        public:
            using CtxPtr = std::shared_ptr<asio::execution_context>;
        public:
            Pipeline(const std::string & name, CtxPtr exec_ctx = nullptr);
            void run();
            void stop();
            [[nodiscard]] auto await(close_chan & close_ch = INVALID_CLOSE_CHAN) -> asio::awaitable<bool>;
            void add_node(const std::string & name, const std::string & type);
            [[nodiscard]] auto link(const std::string & src, const std::string & target, close_chan & close_ch = INVALID_CLOSE_CHAN) -> asio::awaitable<LinkPtr>;
            [[nodiscard]] auto link(const std::string & src, const std::string & src_pad, const std::string & tgt, const std::string & tgt_pad, close_chan & close_ch = INVALID_CLOSE_CHAN) -> asio::awaitable<LinkPtr>;
            [[nodiscard]] GstElementSPtr node(const std::string & name) const;
            [[nodiscard]] GstElementSPtr require_node(const std::string & name) const;
            [[nodiscard]] const char * name() const noexcept;
            [[nodiscard]] const CtxPtr exec_ctx() const noexcept;
        };
    } // namespace gst
} // namespace cfgo



#endif