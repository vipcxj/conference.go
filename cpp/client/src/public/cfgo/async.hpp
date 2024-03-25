#ifndef _CFGO_ASYNC_HPP_
#define _CFGO_ASYNC_HPP_

#include <chrono>
#include <mutex>
#include <vector>
#include "cfgo/alias.hpp"
#include "asio/awaitable.hpp"
#include "asio/steady_timer.hpp"
#include "asiochan/asiochan.hpp"

namespace cfgo
{
    class CloseSignal;
    using close_chan = CloseSignal;
    using close_chan_ptr = std::shared_ptr<close_chan>;
    extern close_chan INVALID_CLOSE_CHAN;
    using duration_t = std::chrono::steady_clock::duration;

    class AsyncMutex {
    private:
        bool m_busy = false;
        using busy_chan = asiochan::channel<void>;
        std::vector<busy_chan> m_busy_chans;
        std::mutex m_mutex;
    public:
        [[nodiscard]] asio::awaitable<bool> accquire(close_chan& close_chan = INVALID_CLOSE_CHAN);
        auto release() -> asio::awaitable<void>;
        void release(asio::execution::executor auto executor)
        {
            std::lock_guard g(m_mutex);
            m_busy = false;
            for (auto &ch : m_busy_chans)
            {
                asio::co_spawn(executor, ch.write(), asio::detached);
            }
        }
    };

    class CloseSignalState;

    class CloseSignal : public asiochan::channel<void, 1>, public std::enable_shared_from_this<CloseSignal>
    {
        using PC = asiochan::channel<void, 1>;
    private:
        std::shared_ptr<CloseSignalState> m_state;
        // void clear_waiter();
    public:
        CloseSignal();
        [[nodiscard]] auto try_read() -> bool;
        [[nodiscard]] auto try_write() -> bool;
        [[nodiscard]] auto read() -> asio::awaitable<void>;
        [[nodiscard]] auto write() -> asio::awaitable<void>;
        [[nodiscard]] bool is_closed() const noexcept;
        void close();
        void set_timeout(const duration_t& dur);

        friend class CloseSignalState;
    };

    inline bool is_valid_close_chan(const close_chan ch) noexcept {
        return ch != INVALID_CLOSE_CHAN;
    }

    template<typename T>
    class cancelable {
        std::variant<T, bool> m_value;

    public:
        cancelable(T value): m_value(value) {}
        cancelable(): m_value(false) {}

        T & value() {
            return std::get<T>(m_value);
        }

        const T & value() const {
            return std::get<T>(m_value);
        }

        bool is_canceled() const noexcept {
            return m_value.index() == 1;
        }

        operator bool() const noexcept {
            return !is_canceled();
        }
    };

    template<>
    class cancelable<bool> {
        bool m_value;
        bool m_canceled;

    public:
        cancelable(bool value): m_value(value), m_canceled(false) {}
        cancelable(): m_value(false), m_canceled(true) {}

        bool value() const {
            return m_value;
        }

        bool is_canceled() const noexcept {
            return m_canceled;
        }

        operator bool() const noexcept {
            return !is_canceled();
        }
    };

    template<>
    class cancelable<void> {
        bool m_canceled;

    public:
        cancelable(bool canceled = true): m_canceled(canceled) {}

        bool is_canceled() const noexcept {
            return m_canceled;
        }

        operator bool() const noexcept {
            return !is_canceled();
        }
    };

    template<typename T>
    cancelable<T> make_resolved(T value) {
        return cancelable<T>(value);
    }

    cancelable<void> make_resolved();

    template<typename T>
    cancelable<T> make_canceled() {
        return cancelable<T>();
    }

    cancelable<void> make_canceled();

    close_chan make_timeout(const duration_t& dur);

    auto wait_timeout(const duration_t& dur) -> asio::awaitable<void>;

    template <asiochan::select_op... Ops,
              asio::execution::executor Executor = typename asiochan::detail::head_t<Ops...>::executor_type>
    requires asiochan::waitable_selection<Ops...>
    [[nodiscard]] auto select(Ops... ops_args) -> asio::awaitable<asiochan::select_result<Ops...>, Executor>
    {

    }

    template<typename T>
    auto chan_read(asiochan::readable_channel_type<T> auto ch, close_chan close_ch = INVALID_CLOSE_CHAN) -> asio::awaitable<cancelable<T>> {
        if (is_valid_close_chan(close_ch))
        {
            if constexpr(std::is_same_v<void, std::decay_t<T>>)
            {
                auto && res = co_await asiochan::select(
                    asiochan::ops::read(ch, close_ch),
                    asiochan::ops::write
                );
                if (res.received_from(close_ch))
                {
                    co_return make_canceled<T>();
                }
                else
                {
                    co_return make_resolved();
                }
            }
            else
            {
                auto && res = co_await asiochan::select(
                    asiochan::ops::read(ch),
                    asiochan::ops::read(close_ch)
                );
                if (res.received_from(close_ch))
                {
                    co_return make_canceled<T>();
                }
                else
                {
                    co_return res.template get_received<T>();
                }
            }
        }
        else
        {
            if constexpr(std::is_same_v<void, std::decay_t<T>>)
            {
                co_await ch.read();
                co_return make_resolved();
            }
            else
            {
                auto && v = co_await ch.read();
                co_return make_resolved(v);
            }
        }
    }
    
} // namespace cfgo


#endif