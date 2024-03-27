#include "cfgo/gst/link.hpp"
#include "impl/link.hpp"

namespace cfgo
{
    namespace gst
    {
        Link::Link(impl_ptr<impl::Link> impl):ImplBy(impl) {}

        const std::string Link::src_pad_name() const noexcept
        {
            return impl()->src_pad_name();
        }

        const std::string Link::tgt_pad_name() const noexcept
        {
            return impl()->tgt_pad_name();
        }
    } // namespace gst
} // namespace cfgo
