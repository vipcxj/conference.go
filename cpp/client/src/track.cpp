#include "cfgo/track.hpp"
#include "impl/track.hpp"

namespace cfgo
{
    Track::Track(const msg_ptr & msg, int cache_capicity): ImplBy<impl::Track>(msg, cache_capicity) {}

    const std::string& Track::type() const noexcept {
        return impl()->type;
    }
    const std::string& Track::pub_id() const noexcept {
        return impl()->pubId;
    }
    const std::string& Track::global_id() const noexcept {
        return impl()->globalId;
    }
    const std::string& Track::bind_id() const noexcept {
        return impl()->bindId;
    }
    const std::string& Track::rid() const noexcept {
        return impl()->rid;
    }
    const std::string& Track::stream_id() const noexcept {
        return impl()->streamId;
    }
    std::map<std::string, std::string> & Track::labels() noexcept {
        return impl()->labels;
    }
    const std::map<std::string, std::string> & Track::labels() const noexcept {
        return impl()->labels;
    }
    std::shared_ptr<rtc::Track> & Track::track() noexcept {
        return impl()->track;
    }
    const std::shared_ptr<rtc::Track> & Track::track() const noexcept {
        return impl()->track;
    }
    auto Track::await_open_or_closed(const close_chan & close_ch) -> asio::awaitable<bool>
    {
        return impl()->await_open_or_closed(close_ch);
    }
    auto Track::await_msg(MsgType msg_type, const close_chan &  close_ch) -> asio::awaitable<MsgPtr>
    {
        return impl()->await_msg(msg_type, close_ch);
    }
    Track::MsgPtr Track::receive_msg(MsgType msg_type) {
        return impl()->receive_msg(msg_type);
    }
    void * Track::get_gst_caps(int pt) const
    {
        return impl()->get_gst_caps(pt);
    }

} // namespace cfgo
