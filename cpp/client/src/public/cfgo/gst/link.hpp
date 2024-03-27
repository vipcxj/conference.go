#ifndef _GST_CFGO_GST_LINK_H_
#define _GST_CFGO_GST_LINK_H_

#include <memory>
#include "cfgo/utils.hpp"

namespace cfgo
{
    namespace gst
    {
        namespace impl
        {
            class Link;
        } // namespace impl

        class Link : ImplBy<impl::Link>
        {
            Link(impl_ptr<impl::Link> impl);
            friend class Pipeline;
        public:
            const std::string src_pad_name() const noexcept;
            const std::string tgt_pad_name() const noexcept;
        };
        using LinkPtr = std::shared_ptr<Link>;
    } // namespace gst
} // namespace cfgo


#endif