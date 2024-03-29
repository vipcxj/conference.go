#ifndef _CFGO_ASYNC_HPP_
#define _CFGO_ASYNC_HPP_

#include <chrono>
#include <mutex>
#include <vector>
#include "cfgo/alias.hpp"
#include "cfgo/common.hpp"
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

    namespace detail
    {
        class CloseSignalState;
    } // namespace detail

    class TimeoutError : public std::exception
    {
    public:
        const char* what() const noexcept override;
    };
    
    class CancelError : public std::exception
    {
    public:
        const char* what() const noexcept override;
    };

    class CloseSignal
    {
    private:
        std::shared_ptr<detail::CloseSignalState> m_state;
        CloseSignal(const std::shared_ptr<detail::CloseSignalState> & state);
        CloseSignal(std::shared_ptr<detail::CloseSignalState> && state);
    public:
        using Waiter = asiochan::channel<void, 1>;
        CloseSignal();
        [[nodiscard]] bool is_closed() const noexcept;
        [[nodiscard]] bool is_timeout() const noexcept;
        void close();
        bool close_no_except() noexcept;
        /**
         * Async wait until closed or timeout. Return false if timeout.
        */
        [[nodiscard]] auto await() -> asio::awaitable<bool>;
        void set_timeout(const duration_t& dur);
        duration_t get_timeout() const noexcept;
        [[nodiscard]] CloseSignal create_child();
        void stop(bool stop_timer = true);
        void resume();
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
        [[nodiscard]] auto get_stop_waiter() -> std::optional<Waiter>;

        friend class detail::CloseSignalState;
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

        auto value() & -> T & {
            return std::get<T>(m_value);
        }

        auto value() const & -> T const & {
            return std::get<T>(m_value);
        }

        auto value() && -> T && {
            return std::get<T>(std::move(m_value));
        }

        auto value() const && -> T const && {
            return std::get<T>(std::move(m_value));
        }

        bool is_canceled() const noexcept {
            return m_value.index() == 1;
        }

        operator bool() const noexcept {
            return !is_canceled();
        }

        inline const T * operator->() const noexcept
        {
            return &value();
        }

        inline T * operator->() noexcept
        {
            return &value();
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

        inline const bool * operator->() const noexcept
        {
            return &m_value;
        }

        inline bool * operator->() noexcept
        {
            return &m_value;
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

    template<typename... TS>
    class cancelable_select_result : public cancelable<std::variant<TS...>>
    {
    public:
        using PC = cancelable<std::variant<TS...>>;
        cancelable_select_result(std::variant<TS...> v): PC(v) {}
        cancelable_select_result(): PC() {}

        template <typename T>
        static constexpr bool is_alternative = (std::same_as<T, TS> or ...);

        // clang-format off
        template <typename T>
        requires is_alternative<T>
        [[nodiscard]] auto get() & -> T&
        // clang-format on
        {
            return std::visit(
                asiochan::detail::overloaded{
                    [](T& result) -> T& { return result; },
                    [](auto const&) -> T& { throw asiochan::bad_select_result_access{}; },
                },
                PC::value());
        }

        // clang-format off
        template <typename T>
        requires is_alternative<T>
        [[nodiscard]] auto get() const& -> T const&
        // clang-format on
        {
            return std::visit(
                asiochan::detail::overloaded{
                    [](T const& result) -> T const& { return result; },
                    [](auto const&) -> T const& { throw asiochan::bad_select_result_access{}; },
                },
                PC::value());
        }

        // clang-format off
        template <typename T>
        requires is_alternative<T>
        [[nodiscard]] auto get() && -> T&&
        // clang-format on
        {
            return std::visit(
                asiochan::detail::overloaded{
                    [](T& result) -> T&& { return std::move(result); },
                    [](auto const&) -> T&& { throw asiochan::bad_select_result_access{}; },
                },
                PC::value());
        }

        // clang-format off
        template <typename T>
        requires is_alternative<T>
        [[nodiscard]] auto get() const&& -> T const&&
        // clang-format on
        {
            return std::visit(
                asiochan::detail::overloaded{
                    [](T const& result) -> T const&& { return std::move(result); },
                    [](auto const&) -> T const&& { throw asiochan::bad_select_result_access{}; },
                },
                PC::value());
        }

        // clang-format off
        template <asiochan::sendable_value T>
        requires is_alternative<asiochan::read_result<T>>
        [[nodiscard]] auto get_received() & -> T&
        // clang-format on
        {
            return get<asiochan::read_result<T>>().get();
        }

        // clang-format off
        template <asiochan::sendable_value T>
        requires is_alternative<asiochan::read_result<T>>
        [[nodiscard]] auto get_received() const& -> T const&
        // clang-format on
        {
            return get<asiochan::read_result<T>>().get();
        }

        // clang-format off
        template <asiochan::sendable_value T>
        requires is_alternative<asiochan::read_result<T>>
        [[nodiscard]] auto get_received()&& -> T&&
        // clang-format on
        {
            return std::move(get<asiochan::read_result<T>>().get());
        }

        // clang-format off
        template <asiochan::sendable_value T>
        requires is_alternative<asiochan::read_result<T>>
        [[nodiscard]] auto get_received() const&& -> T const&&
        // clang-format on
        {
            return std::move(get<asiochan::read_result<T>>().get());
        }

        // clang-format off
        template <asiochan::any_readable_channel_type T>
        requires is_alternative<asiochan::read_result<typename T::send_type>>
        [[nodiscard]] auto received_from(T const& channel) const noexcept -> bool
        // clang-format on
        {
            using SendType = typename T::send_type;

            return std::visit(
                asiochan::detail::overloaded{
                    [&](asiochan::read_result<SendType> const& result)
                    { return result.matches(channel); },
                    [](auto const&)
                    { return false; },
                },
                PC::value());
        }
    };

    template<asiochan::select_op... Ops>
    auto make_canceled_select_result() -> cancelable_select_result<typename Ops::result_type...>
    {
        return cancelable_select_result<typename Ops::result_type...>();
    }

    close_chan make_timeout(const duration_t& dur);

    auto wait_timeout(const duration_t& dur) -> asio::awaitable<void>;

    template <asiochan::sendable T, asiochan::select_op Op1, asiochan::select_op Op2>
    class combine_read_op
    {
    public:
        using executor_type = typename Op1::executor_type;
        using result_type = asiochan::read_result<T>;
        static constexpr auto num_alternatives_1 = Op1::num_alternatives;
        static constexpr auto num_alternatives_2 = Op2::num_alternatives;
        static constexpr auto num_alternatives = num_alternatives_1 + num_alternatives_2;
        static constexpr auto always_waitfree_1 = Op1::always_waitfree;
        static constexpr auto always_waitfree_2 = Op2::always_waitfree;
        static constexpr auto always_waitfree = always_waitfree_1 || always_waitfree_2;
        using wait_state_type_1 = typename Op1::wait_state_type;
        using wait_state_type_2 = typename Op2::wait_state_type;
        struct wait_state_type
        {
            wait_state_type_1 state_1;
            wait_state_type_2 state_2;
        };

        explicit combine_read_op(const Op1 & op1, const Op2 & op2): m_op1(op1), m_op2(op2)
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
            if (auto res = m_op1.submit_with_wait(select_ctx, base_token, wait_state.state_1))
            {
                return res;
            }
            if (auto res = m_op2.submit_with_wait(select_ctx, base_token, wait_state.state_2))
            {
                return Op1::num_alternatives + *res;
            }
            return std::nullopt;
        }

        void clear_wait(
            std::optional<std::size_t> const successful_alternative,
            wait_state_type& wait_state)
        {
            if (successful_alternative)
            {
                m_op1.clear_wait(successful_alternative, wait_state.state_1);
                if (successful_alternative >= Op1::num_alternatives)
                {
                    m_op2.clear_wait(*successful_alternative - Op1::num_alternatives, wait_state.state_2);
                }
                else
                {
                    m_op2.clear_wait(*successful_alternative + num_alternatives, wait_state.state_2);
                }
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

    template<asiochan::select_op Op, asiochan::select_op... Ops>
    constexpr bool none_is_void_read_op()
    {
        if constexpr (sizeof...(Ops) == 0)
            return !is_void_read_op<Op>;
        else
            return !is_void_read_op<Op> && none_is_void_read_op<Ops...>();
    }

    template <asiochan::select_op First_Op, asiochan::select_op... Ops,
              asio::execution::executor Executor = typename First_Op::executor_type>
    requires asiochan::waitable_selection<First_Op, Ops...>
    constexpr auto select_(First_Op first_op, Ops... other_ops)
    {
        if constexpr (sizeof...(Ops) == 0)
        {
            return asiochan::select(first_op);
        }
        else
        {
            return asiochan::select(first_op, other_ops...);
        }
    }
    

    template <asiochan::select_op First_Op, asiochan::select_op... Ops,
              asio::execution::executor Executor = typename First_Op::executor_type>
    requires asiochan::waitable_selection<First_Op, Ops...>
    [[nodiscard]] auto select(close_chan close_ch, First_Op first_op, Ops... other_ops) -> asio::awaitable<cancelable_select_result<typename First_Op::result_type, typename Ops::result_type...>>
    {
        if constexpr (sizeof...(Ops) > 0)
        {
            static_assert(none_is_void_read_op<Ops...>(), "None of other_ops could be asiochan::ops::read<void>, use first_op instead.");
        }
        if (is_valid_close_chan(close_ch))
        {
            co_await close_ch.init_timer();
            if (auto waiter_opt = close_ch.get_waiter())
            {
                if constexpr (is_void_read_op<First_Op>)
                {   
                    auto && res = co_await select_(
                        combine_read_op<void, asiochan::ops::read<void, close_chan::Waiter>, std::decay_t<First_Op>>(
                            asiochan::ops::read(*waiter_opt),
                            first_op
                        ),
                        other_ops...
                    );
                    if (res.received_from(*waiter_opt))
                    {
                        co_return make_canceled_select_result<First_Op, Ops...>();
                    }
                    else
                    {
                        if (auto stop_waiter = close_ch.get_stop_waiter())
                        {
                            co_await stop_waiter->read();
                        }
                        if (close_ch.is_closed() && !close_ch.is_timeout())
                        {
                            co_return make_canceled_select_result<First_Op, Ops...>();
                        }
                        co_return cancelable_select_result<typename First_Op::result_type, typename Ops::result_type...>(std::move(res).to_variant());
                    }
                }
                else
                {
                    auto res = co_await select_(
                        asiochan::ops::read(*waiter_opt),
                        first_op,
                        other_ops...
                    );
                    if (res.received_from(*waiter_opt))
                    {
                        co_return make_canceled_select_result<First_Op, Ops...>();
                    }
                    else
                    {
                        if (auto stop_waiter = close_ch.get_stop_waiter())
                        {
                            co_await stop_waiter->read();
                        }
                        if (close_ch.is_closed() && !close_ch.is_timeout())
                        {
                            co_return make_canceled_select_result<First_Op, Ops...>();
                        }
                        co_return cancelable_select_result<typename First_Op::result_type, typename Ops::result_type...>(
                            magic::shift_variant(std::move(res).to_variant())
                        );
                    }
                }
            }
            else
            {
                co_return make_canceled_select_result<First_Op, Ops...>();
            }
        }
        else
        {
            auto && res = co_await select_(first_op, std::forward<Ops>(other_ops)...);
            co_return cancelable_select_result<typename First_Op::result_type, typename Ops::result_type...>(std::move(res).to_variant());
        }
    }

    template <asiochan::select_op First_Op, asiochan::select_op... Ops,
              asio::execution::executor Executor = typename First_Op::executor_type>
    requires asiochan::waitable_selection<First_Op, Ops...>
    [[nodiscard]] auto select_or_throw(close_chan close_ch, First_Op first_op, Ops... other_ops) -> asio::awaitable<asiochan::select_result<typename First_Op::result_type, typename Ops::result_type...>>
    {
        if constexpr (sizeof...(Ops) > 0)
        {
            static_assert(none_is_void_read_op<Ops...>(), "None of other_ops could be asiochan::ops::read<void>, use first_op instead.");
        }
        if (is_valid_close_chan(close_ch))
        {
            co_await close_ch.init_timer();
            if (auto waiter_opt = close_ch.get_waiter())
            {
                if constexpr (is_void_read_op<First_Op>)
                {   
                    auto && res = co_await select_(
                        combine_read_op<void, asiochan::ops::read<void, close_chan::Waiter>, std::decay_t<First_Op>>(
                            asiochan::ops::read(*waiter_opt),
                            first_op
                        ),
                        other_ops...
                    );
                    if (res.received_from(*waiter_opt))
                    {
                        if (close_ch.is_timeout())
                        {
                            throw TimeoutError();
                        }
                        else
                        {
                            throw CancelError();
                        }
                    }
                    else
                    {
                        if (auto stop_waiter = close_ch.get_stop_waiter())
                        {
                            co_await stop_waiter->read();
                        }
                        if (close_ch.is_closed() && !close_ch.is_timeout())
                        {
                            throw CancelError();
                        }
                        co_return res;
                    }
                }
                else
                {
                    auto res = co_await select_(
                        asiochan::ops::read(*waiter_opt),
                        first_op,
                        other_ops...
                    );
                    if (res.received_from(*waiter_opt))
                    {
                        if (close_ch.is_timeout())
                        {
                            throw TimeoutError();
                        }
                        else
                        {
                            throw CancelError();
                        }
                    }
                    else
                    {
                        if (auto stop_waiter = close_ch.get_stop_waiter())
                        {
                            co_await stop_waiter->read();
                        }
                        if (close_ch.is_closed() && !close_ch.is_timeout())
                        {
                            throw CancelError();
                        }
                        co_return asiochan::select_result(magic::shift_variant(std::move(res).to_variant()));
                    }
                }
            }
            else
            {
                co_return make_canceled_select_result<First_Op, Ops...>();
            }
        }
        else
        {
            auto && res = co_await select_(first_op, std::forward<Ops>(other_ops)...);
            co_return cancelable_select_result<typename First_Op::result_type, typename Ops::result_type...>(std::move(res).to_variant());
        }
    }

    template<typename T>
    auto chan_read(asiochan::readable_channel_type<T> auto ch, close_chan close_ch = INVALID_CLOSE_CHAN) -> asio::awaitable<cancelable<T>> {
        if (is_valid_close_chan(close_ch))
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
                if constexpr(std::is_void_v<T>)
                {
                    co_return make_resolved();
                }
                else
                {
                    co_return res.template get_received<T>();
                }
            }
        }
        else
        {
            if constexpr(std::is_void_v<T>)
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

    template<typename T>
    auto async_retry(const TryOption & option, std::function<asio::awaitable<T>(int)> func, std::function<bool(const T &)> retry_checker, close_chan close_ch) -> asio::awaitable<cancelable<T>>
    {
        auto tried = option.m_tries, tries = option.m_tries;
        auto delay_init = option.m_delay_init;
        auto delay_step = option.m_delay_step;
        auto delay_level = option.m_delay_level > 16 ? 16 : option.m_delay_level;
        std::uint32_t delay_current_level = 0;
        do
        {
            auto res = co_await func(tries - tried + 1);
            if (retry_checker(res))
            {
                if (tried == 0)
                {
                    co_return make_canceled<T>();
                }
                else if (tried > 0)
                {
                    --tried;
                }
                auto delay = delay_init;
                if (delay_current_level > 0)
                {
                    delay += delay_step * (1 << (delay_current_level - 1));
                }
                if (delay_current_level < delay_level)
                {
                    ++delay_current_level;
                }
                if (delay > 0)
                {
                    auto timer = close_ch.create_child();
                    timer.set_timeout(std::chrono::milliseconds {delay});
                    bool closed = co_await timer.await();
                    if (closed)
                    {
                        co_return make_canceled<T>();
                    }
                }
            }
            co_return make_resolved<T>(res);
        } while (true);
    }

    namespace detail
    {
        class AsyncParallelTask;
    } // namespace detail

    class AsyncParallelTask : public ImplBy<detail::AsyncParallelTask>
    {
    public:
        using TaskType = std::function<asio::awaitable<void>(close_chan closer)>;
        enum Mode
        {

        };

        AsyncParallelTask(close_chan close_ch);
    };

} // namespace cfgo


#endif