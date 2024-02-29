#include "cfgo/track.hpp"
#include "impl/track.hpp"

namespace cfgo
{
    Track::Track(const msg_ptr & msg): ImplBy<impl::Track>(msg) {}

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
} // namespace cfgo
