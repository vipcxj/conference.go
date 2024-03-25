#ifndef _CFGO_ASYNC_HPP_
#define _CFGO_ASYNC_HPP_

#include <chrono>
#include <mutex>
#include <vector>
#include "cfgo/alias.hpp"
#include "cfgo/black_magic.hpp"
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
    template <asiochan::select_op... Ops>
    class cancelable_select_result;

    class CloseSignal : public std::enable_shared_from_this<CloseSignal>
    {
        using Waiter = asiochan::channel<void, 1>;
    private:
        std::shared_ptr<CloseSignalState> m_state;
        // void clear_waiter();
    public:
        CloseSignal();
        [[nodiscard]] bool is_closed() const noexcept;
        void close();
        void set_timeout(const duration_t& dur);
        [[nodiscard]] friend inline auto operator==(
            CloseSignal const& lhs,
            CloseSignal const& rhs) noexcept -> bool
        {
            return lhs.m_state == rhs.m_state;
        }
        [[nodiscard]] friend auto operator!=(
            CloseSignal const& lhs,
            CloseSignal const& rhs) noexcept -> bool = default;

        auto init_timer() -> asio::awaitable<void>;
        [[nodiscard]] auto get_waiter() -> std::optional<Waiter>;

        friend class CloseSignalState;
    };

    inline bool is_valid_close_chan(const close_chan ch) noexcept {
        return ch != INVALID_CLOSE_CHAN;
    }

    template<typename T>
    class cancelable {
        std::variant<T, bool> m_value;

    public:
        using value_t = T;
        cancelable(T value): m_value(value) {}
        cancelable(): m_value(false) {}

        T & value() noexcept {
            return std::get<T>(m_value);
        }

        const T & value() const noexcept {
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

    template <asiochan::select_op... Ops>
    class cancelable_select_result : public cancelable<asiochan::select_result<Ops...>>
    {
    public:
        using value_t = asiochan::select_result<Ops...>;
        cancelable_select_result(): cancelable() {}
        cancelable_select_result(const asiochan::select_result<Ops...> & select_result): cancelable(select_result) {}
        inline const value_t * operator->() const noexcept
        {
            return &this->value();
        }
        inline value_t * operator->() noexcept
        {
            return &this->value();
        }
    };

    template <asiochan::sendable T, asiochan::select_op Op1, asiochan::select_op Op2>
    class combine_read_op
    {
    public:
        using executor_type = Op1::executor_type;
        using result_type = asiochan::read_result<T>;
        static constexpr auto num_alternatives = Op1::num_alternatives + Op2::num_alternatives;
        static constexpr auto always_waitfree = Op1::always_waitfree || Op2::always_waitfree;
        using wait_state_type = Op1::wait_state_type;

        explicit combine_read_op(Op1 && op1, Op2 && op2): m_op1(std::forward(op1)), m_op2(std::forward(op2))
        {}

        [[nodiscard]] auto submit_if_ready() -> std::optional<std::size_t>
        {
            if (auto res = m_op1.submit_if_ready())
            {
                return res;
            }
            if (auto res = m_op2.submit_if_ready())
            {
                return Op1::num_alternatives + *res;
            }
            return std::nullopt;
        }

        [[nodiscard]] auto submit_with_wait(
            asiochan::detail::select_wait_context<executor_type>& select_ctx,
            asiochan::detail::select_waiter_token const base_token,
            wait_state_type& wait_state)
            -> std::optional<std::size_t>
        {
            if (auto res = m_op1.submit_with_wait(select_ctx, base_token, wait_state))
            {
                return res;
            }
            if (auto res = m_op2.submit_with_wait(select_ctx, base_token, wait_state))
            {
                return Op1::num_alternatives + *res;
            }
            return std::nullopt;
        }

        void clear_wait(
            std::optional<std::size_t> const successful_alternative,
            wait_state_type& wait_state)
        {
            m_op1.clear_wait(successful_alternative, wait_state);
            if (successful_alternative >= Op1::num_alternatives)
            {
                m_op2.clear_wait(successful_alternative - Op1::num_alternatives, wait_state);
            }
            else
            {
                m_op2.clear_wait(successful_alternative + num_alternatives, wait_state);
            }
        }

        [[nodiscard]] auto get_result(std::size_t const successful_alternative) noexcept -> result_type
        {
            if (successful_alternative >= Op1::num_alternatives)
            {
                return m_op2.get_result(successful_alternative - Op1::num_alternatives);
            }
            else
            {
                return m_op1.get_result(successful_alternative);
            }
        }

    private:
        Op1 m_op1;
        Op2 m_op2;
    };

    template<asiochan::select_op Op>
    constexpr bool is_void_read_op = std::is_same_v<asiochan::read_result<void>, std::decay_t<typename Op::result_type>>;

    template <asiochan::select_op... Ops,
              asio::execution::executor Executor = typename asiochan::detail::head_t<Ops...>::executor_type>
    requires asiochan::waitable_selection<Ops...>
    [[nodiscard]] auto select(close_chan close_ch, Ops... ops_args) -> asio::awaitable<cancelable_select_result<Ops...>, Executor>
    {
        if (is_valid_close_chan(close_ch))
        {
            co_await close_ch.init_timer();
            if (auto waiter_opt = close_ch.get_waiter())
            {
                auto void_read_op = std::tuple_cat(magic::pick_if<is_void_read_op<Ops>>(std::forward<Ops>(ops_args))...);
                auto other_ops = std::tuple_cat(magic::pick_if<!is_void_read_op<Ops>>(std::forward<Ops>(ops_args))...);
                constexpr auto void_read_op_num = std::tuple_size_v<decltype(void_read_op)>;
                static_assert(void_read_op_num == 0 || void_read_op_num == 1, "only accept at most one void read op.");
                if constexpr (std::tuple_size_v<decltype(void_read_op)> == 0)
                {
                    auto res = co_await std::apply(
                        asiochan::select,
                        std::tuple_cat(std::forward_as_tuple<>(std::forward<Ops>(ops_args)...), std::make_tuple(asiochan::ops::read(*waiter_opt)))
                    );
                    if (res.received_from(*waiter_opt))
                    {
                        co_return cancelable_select_result {};
                    }
                    else
                    {
                        co_return cancelable_select_result {asiochan::select_result<Ops...>(std::get(res.to_variant(), res.alternative()), res.alternative())};
                    }
                }
                else
                {
                    auto res = co_await std::apply(
                        asiochan::select,
                        std::tuple_cat(other_ops, std::make_tuple(combine_read_op<void, auto, auto>(std::get<0>(void_read_op), asiochan::ops::read(*waiter_opt))))
                    );
                    // TO-DO:
                    co_return cancelable_select_result {};
                }
            }
            else
            {
                co_return cancelable_select_result {};
            }
        }
        else
        {
            auto res = asiochan::select(std::forward<Ops>(ops_args)...);
            co_return cancelable_select_result{res};
        }

    }

    template<typename T>
    auto chan_read(asiochan::readable_channel_type<T> auto ch, close_chan close_ch = INVALID_CLOSE_CHAN) -> asio::awaitable<cancelable<T>> {
        if (is_valid_close_chan(close_ch))
        {
            if constexpr(std::is_same_v<void, std::decay_t<T>>)
            {
                auto && res = co_await select(
                    close_ch,
                    asiochan::ops::read(ch)
                );
                if (!res)
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
                auto && res = co_await select(
                    close_ch,
                    asiochan::ops::read(ch)
                );
                if (!res)
                {
                    co_return make_canceled<T>();
                }
                else
                {
                    co_return res->get_received<T>();
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