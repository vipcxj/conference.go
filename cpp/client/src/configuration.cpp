#include "cfgo/configuration.hpp"

namespace cfgo {

    Configuration::Configuration(
        const std::string& signal_url,
        const std::string& token,
        const asio::io_context *io_ctx = nullptr
    ):
    m_signal_url(signal_url),
    m_token(token),
    m_rtc_config(),
    m_io_ctx(io_ctx)
    {}

    Configuration::Configuration(
        const std::string& signal_url,
        const std::string& token,
        const rtc::Configuration& rtc_config,
        const asio::io_context *io_ctx = nullptr
    ):
    m_signal_url(signal_url),
    m_token(token),
    m_rtc_config(rtc_config),
    m_io_ctx(io_ctx)
    {}
}