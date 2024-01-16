#ifndef _CFGO_CONFIGURATION_HPP_
#define _CFGO_CONFIGURATION_HPP_

#include "rtc/rtc.hpp"
#include "asio/io_context.hpp"

namespace cfgo {
    struct Configuration
    {
        const asio::io_context * m_io_ctx;
        const std::string m_signal_url;
        const std::string m_token;
        const rtc::Configuration m_rtc_config;

        Configuration(
            const std::string& signal_url,
            const std::string& token,
            const asio::io_context *io_ctx = nullptr
        );

        Configuration(
            const std::string& signal_url,
            const std::string& token,
            const rtc::Configuration& rtc_config,
            const asio::io_context *io_ctx = nullptr
        );
    };
    
}

#endif