#ifndef _GST_CFGO_GST_LINK_H_
#define _GST_CFGO_GST_LINK_H_

#include <memory>
#include "cfgo/utils.hpp"
#include "cfgo/async.hpp"
#include "asio/awaitable.hpp"

namespace cfgo
{
    namespace gst
    {
        namespace impl
        {
            class Link;
            class AsyncLink;
        } // namespace impl

        class Link : ImplBy<impl::Link>
        {
        public:
            Link(impl_ptr<impl::Link> impl);
            const std::string src_pad_name() const noexcept;
            const std::string tgt_pad_name() const noexcept;

            friend class Pipeline;
            friend class AsyncLink;
        };
        using LinkPtr = std::shared_ptr<Link>;

        class AsyncLink : ImplBy<impl::AsyncLink>
        {
        public:
            AsyncLink(impl_ptr<impl::AsyncLink> impl);
            auto await(const close_chan & closer = INVALID_CLOSE_CHAN) -> asio::awaitable<LinkPtr>;
        };
        using AsyncLinkPtr = std::shared_ptr<AsyncLink>;
    } // namespace gst
} // namespace cfgo


#endif