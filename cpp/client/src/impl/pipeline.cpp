#include "impl/pipeline.hpp"
#include "cpptrace/cpptrace.hpp"
#include "spdlog/spdlog.h"
#include "cfgo/defer.hpp"
#include "cfgo/utils.hpp"
#include <cctype>

namespace cfgo
{
    namespace gst
    {

        namespace impl
        {

            void pad_added_handler(GstElement *src, GstPad *new_pad, Pipeline *pipeline)
            {
                std::lock_guard lock(pipeline->m_mutex);
                std::string node_name = GST_ELEMENT_NAME(src);
                std::string pad_name = GST_PAD_NAME(new_pad);
                auto link_iter = pipeline->find_pending_link(node_name, pad_name, Pipeline::ANY);
                if (link_iter != pipeline->m_pending_links.end())
                {
                    auto link = *link_iter;
                    if (src == link->src())
                    {
                        link->set_src_pad(new_pad);
                    }
                    else
                    {
                        link->set_tgt_pad(new_pad);
                    }
                }
            }

            gboolean on_bus_message(GstBus *bus, GstMessage *message, Pipeline *pipeline) 
            {
                gst_message_ref(message);
                pipeline->m_msg_ch.write(message);
                return TRUE;
            }

            Pipeline::Pipeline(const std::string & name, CtxPtr exec_ctx): m_exec_ctx(exec_ctx)
            {
                if (!m_exec_ctx)
                {
                    m_exec_ctx = std::make_shared<asio::io_context>();
                }
                
                m_pipeline = GST_PIPELINE (gst_pipeline_new(name.c_str()));
                if (!m_pipeline)
                {
                    throw cpptrace::runtime_error("Unable to create the pipeline with name " + name + ".");
                }
                m_bus = gst_element_get_bus(GST_ELEMENT (m_pipeline));
                if (!m_bus)
                {
                    throw cpptrace::runtime_error("Unable to get the bus from the pipeline " + name + ".");
                }
                gst_bus_add_watch(m_bus, (GstBusFunc) on_bus_message, this);
            }

            Pipeline::~Pipeline()
            {
                gst_bus_remove_watch(m_bus);
                while (auto msg = m_msg_ch.try_read())
                {
                    gst_message_unref(*msg);
                }
                gst_object_unref(m_bus);
                stop();
                gst_object_unref(m_pipeline);
            }

            void Pipeline::run()
            {
                auto ret = gst_element_set_state (GST_ELEMENT (m_pipeline), GST_STATE_PLAYING);
                if (ret == GST_STATE_CHANGE_FAILURE)
                {
                    throw cpptrace::runtime_error("Unable to set the pipeline " + std::string(name()) + " to the playing state.");
                }
            }

            void Pipeline::stop()
            {
                gst_element_set_state (GST_ELEMENT (m_pipeline), GST_STATE_NULL);
            }

            auto Pipeline::await(close_chan & close_ch) -> asio::awaitable<bool>
            {
                do
                {
                    auto c_msg = co_await chan_read<GstMessage *>(m_msg_ch, close_ch);
                    if (!c_msg)
                    {
                        co_return false;
                    }
                    auto msg = c_msg.value();
                    DEFER({
                        gst_message_unref(c_msg.value());
                    });
                    switch (GST_MESSAGE_TYPE (msg))
                    {
                    case GST_MESSAGE_ERROR:
                    {
                        GError * err;
                        gchar * debug_info;
                        gst_message_parse_error(msg, &err, &debug_info);
                        DEFER({
                            g_clear_error(&err);
                            g_free(debug_info);
                        });
                        throw cpptrace::runtime_error(
                            std::string("Error received from element ")
                            + GST_OBJECT_NAME (msg->src) 
                            + ": " + err->message 
                            + " --- Debug info: " + (debug_info ? debug_info : "none")
                        );
                    }
                    case GST_MESSAGE_EOS:
                        co_return true;
                    default:
                        break;
                    }
                } while (true);
            }

            void Pipeline::add_node(const std::string & name, const std::string & type)
            {
                std::lock_guard lock(m_mutex);
                if (m_nodes.contains(name))
                {
                    throw cpptrace::runtime_error("The node " + name + " (" + type + ") has already exist in the pipeline " + this->name() + ".");
                }
                auto element = gst_element_factory_make(type.c_str(), name.c_str());
                if (!element)
                {
                    throw cpptrace::runtime_error("Unable to create the " + type + " element with name " + name + ".");
                }
                if (!gst_bin_add(GST_BIN (m_pipeline), element))
                {
                    throw cpptrace::runtime_error("Unable add the " + type + " element with name " + name + " to the pipeline " + this->name() + ".");
                }
                if (g_signal_lookup("pad-added", G_OBJECT_TYPE (element)))
                {
                    g_signal_connect(element, "pad-added", G_CALLBACK(pad_added_handler), this);
                }
                
                m_nodes.emplace(std::make_pair(name, element));
            }

            /**
             * only support %u
            */
            bool match_pad_name(const std::string & name1, const std::string & name2)
            {

                auto len1 = name1.length();
                auto len2 = name2.length();
                bool f1 = 0, f2 = 0;
                size_t i = 0, j = 0;
                for (;i < len1 && j < len2; ++i, ++j)
                {
                    auto c1 = name1[i];
                    auto c2 = name2[i];
                    if (f1 > 0)
                    {
                        if (f1 == 1)
                        {
                            if (c1 != 'u')
                            {
                                spdlog::warn("Unsupport format. pad1: {}, pad2: {}.", name1, name2);
                                return false;
                            }
                            else
                            {
                                f1 = 2;
                            }
                        }
                        if (!std::isdigit(c2))
                        {
                            f1 = 0;
                            ++i;
                        }
                        ++j;
                    }
                    else if (f2 > 0)
                    {
                        if (f2 == 1)
                        {
                            if (c2 != 'u')
                            {
                                spdlog::warn("Unsupport format. pad1: {}, pad2: {}.", name1, name2);
                                return false;
                            }
                            else
                            {
                                f2 = 2;
                            }
                        }
                        if (!std::isdigit(c1))
                        {
                            f2 = 0;
                            ++j;
                        }
                        ++i;
                    }
                    else
                    {
                        if (c1 != c2)
                        {
                            if (c1 == '%')
                            {
                                if (!std::isdigit(c2))
                                {
                                    return false;
                                }
                                f1 = 1;
                            }
                            else if (c2 == '%')
                            {
                                if (!std::isdigit(c1))
                                {
                                    return false;
                                }
                                f2 = 1;
                            }
                            else
                            {
                                return false;
                            }
                        }
                        ++i;
                        ++j;
                    }
                }
                return i == len1 && j == len2;
            }

            [[nodiscard]] auto Pipeline::find_pending_link(const std::string & node, const std::string pad, EndpointType endpoint_type) const -> LinkList::const_iterator
            {
                return std::find_if(m_pending_links.begin(), m_pending_links.end(), [endpoint_type, &node, &pad](auto && link) {
                    if (endpoint_type == Pipeline::SRC)
                        return node == GST_ELEMENT_NAME (link->src()) && match_pad_name(pad, link->src_pad_name());
                    else if (endpoint_type == Pipeline::TGT)
                        return node == GST_ELEMENT_NAME (link->tgt()) && match_pad_name(pad, link->tgt_pad_name());
                    else
                        return (node == GST_ELEMENT_NAME (link->src()) && match_pad_name(pad, link->src_pad_name())) || (node == GST_ELEMENT_NAME (link->tgt()) && match_pad_name(pad, link->tgt_pad_name()));     
                });
            }
            [[nodiscard]] auto Pipeline::find_pending_link(const std::string & node, const std::string pad, EndpointType endpoint_type) -> LinkList::iterator
            {
                return std::find_if(m_pending_links.begin(), m_pending_links.end(), [endpoint_type, &node, &pad](auto && link) {
                    if (endpoint_type == Pipeline::SRC)
                        return node == GST_ELEMENT_NAME (link->src()) && match_pad_name(pad, link->src_pad_name());
                    else if (endpoint_type == Pipeline::TGT)
                        return node == GST_ELEMENT_NAME (link->tgt()) && match_pad_name(pad, link->tgt_pad_name());
                    else
                        return (node == GST_ELEMENT_NAME (link->src()) && match_pad_name(pad, link->src_pad_name())) || (node == GST_ELEMENT_NAME (link->tgt()) && match_pad_name(pad, link->tgt_pad_name()));     
                });
            }
            [[nodiscard]] Pipeline::LinkPtr Pipeline::link_by_src(const std::string & node_name, const std::string pad_name) const
            {
                auto iter0 = m_links_by_src.find(node_name);
                if (iter0 != m_links_by_src.end())
                {
                    auto iter1 = iter0->second.find(pad_name);
                    if (iter1 != iter0->second.end())
                    {
                        return iter1->second;
                    }
                }
                auto iter = find_pending_link(node_name, pad_name, Pipeline::SRC);
                return iter != m_pending_links.end() ? *iter : nullptr;
            }

            #define CFGO_UNABLE_TO_LINK_MSG(SRC, SRC_PAD, TGT, TGT_PAD, MSG) \
                "Unable to link from pad " + SRC_PAD + " of " + SRC + " to pad " + TGT_PAD + " of " + TGT + ". " + MSG

            void Pipeline::add_link(const LinkPtr & link, bool clean_pending)
            {
                if (!link->is_ready())
                {
                    throw cpptrace::logic_error(THIS_IS_IMPOSSIBLE);
                }
                std::string src_name = GST_ELEMENT_NAME (link->src());
                std::string src_pad_name = link->src_pad_name();
                if (GST_PAD_LINK_FAILED (gst_pad_link(link->src_pad(), link->tgt_pad())))
                {
                    throw cpptrace::runtime_error(CFGO_UNABLE_TO_LINK_MSG(
                        src_name, src_pad_name, GST_ELEMENT_NAME (link->tgt()), link->tgt_pad_name(), 
                        "Link failed."
                    ));
                }
                if (clean_pending)
                {
                    auto iter = find_pending_link(src_name, src_pad_name, Pipeline::SRC);
                    if (iter != m_pending_links.end())
                    {
                        m_pending_links.erase(iter);
                    }
                }
                if (auto iter = m_links_by_src.find(src_name); iter == m_links_by_src.end())
                {
                    m_links_by_src[src_name] = std::unordered_map<std::string, LinkPtr>();
                }
                m_links_by_src[src_name][src_pad_name] = link;
            }

            bool Pipeline::remove_link(const LinkPtr & link, bool clean_pending)
            {
                std::string src_name = GST_ELEMENT_NAME (link->src());
                std::string src_pad_name = link->src_pad_name();
                if (clean_pending)
                {
                    auto iter = find_pending_link(src_name, src_pad_name, Pipeline::SRC);
                    if (iter != m_pending_links.end())
                    {
                        m_pending_links.erase(iter);
                        return true;
                    }
                }
                auto iter1 = m_links_by_src.find(src_name);
                if (iter1 == m_links_by_src.end())
                {
                    return false;
                }
                auto iter2 = iter1->second.find(src_pad_name);
                if (iter2 == iter1->second.end())
                {
                    return false;
                }
                iter1->second.erase(iter2);
                if (iter1->second.empty())
                {
                    m_links_by_src.erase(iter1);
                }
                return true;
            }

            auto Pipeline::link(const std::string & src, const std::string & target, close_chan & close_chan) -> asio::awaitable<LinkPtr>
            {
                throw cpptrace::runtime_error(NOT_IMPLEMENTED_YET);
            }

            auto Pipeline::link(const std::string & src_name, const std::string & src_pad_name, const std::string & tgt_name, const std::string & tgt_pad_name, close_chan & close_ch) -> asio::awaitable<LinkPtr>
            {
                bool done = false;
                LinkPtr _link = nullptr;
                {
                    std::lock_guard lock(m_mutex);
                    if (link_by_src(src_name, src_pad_name))
                    {
                        throw cpptrace::runtime_error(CFGO_UNABLE_TO_LINK_MSG(
                            src_name, src_pad_name, tgt_name, tgt_pad_name, 
                            "Link already exist."
                        ));
                    }
                    
                    auto src_node = this->require_node(src_name);
                    GstPad * src_pad = gst_element_get_static_pad(src_node, src_pad_name.c_str());
                    if (!src_pad)
                    {
                        auto pad_template = gst_element_get_pad_template(src_node, src_pad_name.c_str());
                        if (!pad_template)
                        {
                            throw cpptrace::runtime_error(CFGO_UNABLE_TO_LINK_MSG(
                                src_name, src_pad_name, tgt_name, tgt_pad_name, 
                                "No pad " + src_pad_name + " found on node " + src_name + "."
                            ));
                        }
                        if (GST_PAD_TEMPLATE_DIRECTION (pad_template) != GST_PAD_SRC)
                        {
                            throw cpptrace::runtime_error(CFGO_UNABLE_TO_LINK_MSG(
                                src_name, src_pad_name, tgt_name, tgt_pad_name, 
                                "The pad " + src_pad_name + " of " + src_name + " is not a src pad."
                            ));
                        }
                        switch (GST_PAD_TEMPLATE_PRESENCE (pad_template))
                        {
                        case GST_PAD_REQUEST:
                            src_pad = gst_element_request_pad_simple(src_node, src_pad_name.c_str());
                            if (!src_pad)
                            {
                                throw cpptrace::runtime_error(CFGO_UNABLE_TO_LINK_MSG(
                                    src_name, src_pad_name, tgt_name, tgt_pad_name, 
                                    "Unable to request the pad " + src_pad_name + " from " + src_name + "."
                                ));
                            }
                            break;
                        case GST_PAD_SOMETIMES:
                            break;
                        case GST_PAD_ALWAYS:
                            throw cpptrace::runtime_error(cfgo::THIS_IS_IMPOSSIBLE);
                        default:
                            throw cpptrace::logic_error(CFGO_UNABLE_TO_LINK_MSG(
                                src_name, src_pad_name, tgt_name, tgt_pad_name, 
                                "Unknown pad presence value: " + std::to_string((int) GST_PAD_TEMPLATE_PRESENCE (pad_template))
                            ));
                        }
                    }
                    auto tgt_node = this->require_node(tgt_name);
                    GstPad * tgt_pad = gst_element_get_static_pad(tgt_node, tgt_pad_name.c_str());
                    if (!tgt_pad)
                    {
                        auto pad_template = gst_element_get_pad_template(tgt_node, tgt_pad_name.c_str());
                        if (!pad_template)
                        {
                            throw cpptrace::runtime_error(CFGO_UNABLE_TO_LINK_MSG(
                                src_name, src_pad_name, tgt_name, tgt_pad_name, 
                                "No pad " + tgt_pad_name + " found on node " + tgt_name + "."
                            ));
                        }
                        if (GST_PAD_TEMPLATE_DIRECTION (pad_template) != GST_PAD_SINK)
                        {
                            throw cpptrace::runtime_error(CFGO_UNABLE_TO_LINK_MSG(
                                src_name, src_pad_name, tgt_name, tgt_pad_name, 
                                "The pad " + tgt_pad_name + " of " + tgt_name + " is not a sink pad."
                            ));
                        }
                        switch (GST_PAD_TEMPLATE_PRESENCE (pad_template))
                        {
                        case GST_PAD_REQUEST:
                            tgt_pad = gst_element_request_pad_simple(tgt_node, tgt_pad_name.c_str());
                            if (!tgt_pad)
                            {
                                throw cpptrace::runtime_error(CFGO_UNABLE_TO_LINK_MSG(
                                    src_name, src_pad_name, tgt_name, tgt_pad_name, 
                                    "Unable to request the pad " + tgt_pad_name + " from " + tgt_name + "."
                                ));
                            }
                            break;
                        case GST_PAD_SOMETIMES:
                            break;
                        case GST_PAD_ALWAYS:
                            throw cpptrace::runtime_error(cfgo::THIS_IS_IMPOSSIBLE);
                        default:
                            throw cpptrace::logic_error(CFGO_UNABLE_TO_LINK_MSG(
                                src_name, src_pad_name, tgt_name, tgt_pad_name, 
                                "Unknown pad presence value: " + std::to_string((int) GST_PAD_TEMPLATE_PRESENCE (pad_template))
                            ));
                        }
                    }
                    _link = std::make_shared<impl::Link>(
                        this,
                        src_node, src_pad_name, src_pad,
                        tgt_node, tgt_pad_name, tgt_pad
                    );
                    done = _link->is_ready();
                    if (done)
                    {
                        add_link(_link, false);
                    }
                    else
                    {
                        m_pending_links.push_back(_link);
                    }
                }
                if (done)
                {
                    co_return _link;
                }
                auto res = co_await _link->wait_ready(close_ch);
                if (!res)
                {
                    {
                        std::lock_guard lock(m_mutex);
                        remove_link(_link, true);
                    }
                    co_return nullptr;
                }
                {
                    std::lock_guard lock(m_mutex);
                    add_link(_link, true);
                }
                co_return _link;
            }

            GstElement * Pipeline::node(const std::string & name) const
            {
                auto && iter = m_nodes.find(name);
                return iter != m_nodes.end() ? iter->second : nullptr;
            }

            GstElement * Pipeline::require_node(const std::string & name) const
            {
                auto node = this->node(name);
                if (!node)
                {
                    throw cpptrace::runtime_error("Unable to find the node " + name + ".");
                }
                return node;
            }


        } // namespace impl
  
    } // namespace gst  
} // namespace cfgo
