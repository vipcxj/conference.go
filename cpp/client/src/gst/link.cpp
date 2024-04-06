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

        AsyncLink::AsyncLink(impl_ptr<impl::AsyncLink> impl): ImplBy(impl) {}

        auto AsyncLink::await(const close_chan & closer) -> asio::awaitable<LinkPtr>
        {
            auto ptr = co_await impl()->await(closer);
            if (ptr)
            {
                co_return std::make_shared<Link>(ptr);
            }
            else
            {
                co_return nullptr;
            }
        }
    } // namespace gst
} // namespace cfgo
