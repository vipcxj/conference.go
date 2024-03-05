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
    Track::MsgPtr Track::receive_msg() const {
        return impl()->receive_msg();
    }

} // namespace cfgo
