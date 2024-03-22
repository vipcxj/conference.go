#ifndef _CFGO_GST_PIPELINE_IMPL_HPP_
#define _CFGO_GST_PIPELINE_IMPL_HPP_

#include <memory>
#include <unordered_map>
#include <list>
#include "asio.hpp"
#include "gst/gst.h"
#include "cfgo/alias.hpp"
#include "cfgo/gst/link.hpp"
#include "impl/link.hpp"

namespace cfgo
{
    namespace impl
    {
        class Pipeline
        {
        public:
            using CtxPtr = std::shared_ptr<asio::execution_context>;
            using NODE_MAP = std::unordered_map<std::string, GstElement *>;
            using LinkPtr = std::shared_ptr<Link>;
            using LinkList = std::list<LinkPtr>;
            using LinkMap = std::unordered_map<std::string, std::unordered_map<std::string, LinkPtr>>;
            enum EndpointType
            {
                SRC,
                TGT,
                ANY
            };
        private:
            CtxPtr m_exec_ctx;
            GstPipeline * m_pipeline;
            GstBus * m_bus;
            NODE_MAP m_nodes;
            LinkMap m_links_by_src;
            LinkList m_pending_links;
            std::mutex m_mutex;
            asiochan::unbounded_channel<GstMessage *> m_msg_ch;

            [[nodiscard]] auto find_pending_link(const std::string & node, const std::string pad, EndpointType endpoint_type) const -> LinkList::const_iterator;
            [[nodiscard]] auto find_pending_link(const std::string & node, const std::string pad, EndpointType endpoint_type) -> LinkList::iterator;
            [[nodiscard]] LinkPtr link_by_src(const std::string & node_name, const std::string pad_name) const;
            void add_link(const LinkPtr & link, bool clean_pending);
            bool remove_link(const LinkPtr & link, bool clean_pending);
        public:
            Pipeline(const std::string & name, CtxPtr exec_ctx = nullptr);
            ~Pipeline();
            void add_node(const std::string & name, const std::string & type);
            void run();
            void stop();
            [[nodiscard]] auto await(close_chan & close_ch) -> asio::awaitable<bool>;
            [[nodiscard]] GstElement * node(const std::string & name) const;
            [[nodiscard]] GstElement * require_node(const std::string & name) const;
            [[nodiscard]] auto link(const std::string & src, const std::string & target, close_chan & close_chan) -> asio::awaitable<LinkPtr>;
            [[nodiscard]] auto link(const std::string & src, const std::string & src_pad, const std::string & tgt, const std::string & tgt_pad, close_chan & close_chan) -> asio::awaitable<LinkPtr>;
            [[nodiscard]] inline const char * name() const noexcept
            {
                return GST_ELEMENT_NAME(m_pipeline);
            }
            [[nodiscard]] inline const CtxPtr exec_ctx() const noexcept
            {
                return m_exec_ctx;
            }

            friend void pad_added_handler(GstElement *src, GstPad *new_pad, Pipeline *pipeline);
            friend gboolean on_bus_message(GstBus *bus, GstMessage *message, Pipeline *pipeline);
        };
    } // namespace impl
    
} // namespace cfgo


#endif