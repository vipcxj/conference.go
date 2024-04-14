#include "cfgo/async_locker.hpp"
#include "cfgo/async.hpp"
#include <list>
#include <memory>

namespace cfgo
{
    namespace detail
    {
        class AsyncBlocker
        {
        public:
            AsyncBlocker(std::uint32_t id): m_id(id) {}
            AsyncBlocker(const AsyncBlocker &) = delete;
            AsyncBlocker & operator = (const AsyncBlocker &) = delete;
            auto request_block(close_chan closer) -> asio::awaitable<bool>;
            auto await_unblock() -> asio::awaitable<void>;
            void unblock();
            std::uint32_t id() const noexcept;
        private:
            std::uint32_t m_id;
            bool m_block = false;
            bool m_blocked = false;
            unique_void_chan m_request_chan;
            unique_void_chan m_response_chan;
        };

        // can not concurrent with unblock
        auto AsyncBlocker::request_block(close_chan closer) -> asio::awaitable<bool>
        {
            if (m_block)
            {
                throw cpptrace::logic_error("request_block could not be called when already requested block.");
            }
            
            m_block = true;
            do
            {
                if (!co_await chan_read<void>(m_response_chan, closer))
                {
                    unblock();
                    co_return false;
                }
                if (m_blocked)
                {
                    co_return true;
                }
            } while (true);
        }
        
        // can not concurrent with request_block
        void AsyncBlocker::unblock()
        {
            m_block = false;
            std::ignore = m_request_chan.try_write();
        }

        auto AsyncBlocker::await_unblock() -> asio::awaitable<void>
        {
            if (m_blocked)
            {
                throw cpptrace::logic_error("await_unblock should not be called when already blocked.");
            }
            
            if (!m_block)
            {
                co_return;
            }
            m_blocked = true;
            do
            {
                std::ignore = m_response_chan.try_write();
                co_await m_request_chan.read();
                if (!m_block)
                {
                    m_blocked = false;
                    co_return;
                }
                
            } while (true);
        }

        std::uint32_t AsyncBlocker::id() const noexcept
        {
            return m_id;
        }

        class AsyncBlockerManager
        {
        public:
            using ScheduleConfigure = cfgo::AsyncBlockerManager::Configure;
            struct BlockerRequest
            {
                std::uint32_t m_id;
                int m_priority;
                unique_chan<AsyncBlockerPtr> m_chan;
            };
            
            struct BlockerInfo
            {
                AsyncBlockerPtr m_blocker;
                std::uint32_t m_epoch;
                int m_priority;
                bool m_used;
                bool m_valid;
            };

            AsyncBlockerManager(const ScheduleConfigure & configure);
            AsyncBlockerManager(const AsyncBlockerManager &) = delete;
            AsyncBlockerManager & operator = (const AsyncBlockerManager &) = delete;

            auto lock(close_chan closer) -> asio::awaitable<void>;
            void unlock();
            auto add_blocker(int priority, close_chan closer) -> asio::awaitable<AsyncBlockerPtr>;
        protected:
            auto _select_and_request_block(int n, close_chan closer) -> asio::awaitable<int>;
            int _calc_batch() const noexcept
            {
                int n = m_conf.batch;
                if (n > 0)
                {
                    if (n > m_blockers.size())
                    {
                        n = m_blockers.size();
                    }
                }
                else if (n < 0)
                {
                    n += m_blockers.size();
                    if (n <= 0)
                    {
                        n = 1;
                    }
                }
                else
                {
                    n = 1;
                }
                return n;
            }
        private:
            ScheduleConfigure m_conf;
            std::list<BlockerInfo> m_blockers;
            std::list<BlockerRequest> m_blocker_requests;
            std::uint32_t m_next_id = 0;
            std::uint32_t m_next_epoch = 0;
            bool m_locked = false;
            std::mutex m_mutex;
        };
        
        AsyncBlockerManager::AsyncBlockerManager(const ScheduleConfigure & configure): m_conf(configure) {}
        auto AsyncBlockerManager::add_blocker(int priority, close_chan closer) -> asio::awaitable<AsyncBlockerPtr>
        {
            AsyncBlockerPtr block_ptr = nullptr;
            std::uint32_t id;
            unique_chan<AsyncBlockerPtr> chan{};
            {
                std::lock_guard lk(m_mutex);
                if (!m_locked)
                {
                    block_ptr = std::make_shared<AsyncBlocker>(m_next_id++);
                    m_blockers.push_back({block_ptr, m_next_epoch++, priority, false, true});
                }
                else
                {
                    id = m_next_id++;
                    m_blocker_requests.push_back({id, priority, chan});
                }
            }
            if (block_ptr)
            {
                co_return block_ptr;
            }
            auto c_block_ptr = co_await chan_read<AsyncBlockerPtr>(chan, closer);
            if (c_block_ptr)
            {
                co_return c_block_ptr.value();
            }
            else
            {
                std::lock_guard lk(m_mutex);
                auto req_iter = std::find_if(m_blocker_requests.begin(), m_blocker_requests.end(), [id](const BlockerRequest & request) -> bool {
                    return request.m_id == id;
                });
                if (req_iter != m_blocker_requests.end())
                {
                    m_blocker_requests.erase(req_iter);
                    throw CancelError(closer);
                }
                auto iter = std::find_if(m_blockers.begin(), m_blockers.end(), [id](const BlockerInfo & info) -> bool {
                    return info.m_blocker->id() == id;
                });
                if (iter == m_blockers.end())
                {
                    throw cpptrace::logic_error(THIS_IS_IMPOSSIBLE);
                }
                co_return iter->m_blocker;
            }
        }
        auto AsyncBlockerManager::lock(close_chan closer) -> asio::awaitable<void>
        {
            int n = _calc_batch();

        }
        void AsyncBlockerManager::unlock()
        {
            {
                std::lock_guard lk(m_mutex);
                m_locked = false;
                for (auto && blocker : m_blockers)
                {
                    if (blocker.m_used)
                    {
                        blocker.m_used = false;
                        blocker.m_blocker->unblock();
                    }
                }
                for (auto it = m_blockers.begin(); it != m_blockers.end();)
                {
                    auto & blocker = *it;
                    if (!blocker.m_valid)
                    {
                        it = m_blockers.erase(it);
                    }
                    else
                    {
                        ++it;
                    }
                }
                for (auto && request : m_blocker_requests)
                {
                    auto block_ptr = std::make_shared<AsyncBlocker>(request.m_id);
                    m_blockers.push_back({block_ptr, m_next_epoch++, request.m_priority, false, true});
                    request.m_chan.try_write(block_ptr);
                }
                m_blocker_requests.clear();
            }
        }


    } // namespace detail

    AsyncBlocker::AsyncBlocker(detail::AsyncBlockerPtr impl): ImplBy(impl) {}

    auto AsyncBlocker::request_block(close_chan closer) -> asio::awaitable<bool>
    {
        return impl()->request_block(std::move(closer));
    }

    auto AsyncBlocker::await_unblock() -> asio::awaitable<void>
    {
        return impl()->await_unblock();
    }

    void AsyncBlocker::unblock()
    {
        impl()->unblock();
    }

    std::uint32_t AsyncBlocker::id() const noexcept
    {
        return impl()->id();
    }

    AsyncBlockerManager::AsyncBlockerManager(const Configure & configure): ImplBy(configure) {}

    auto AsyncBlockerManager::lock(close_chan closer) -> asio::awaitable<void>
    {
        return impl()->lock(std::move(closer));
    }

    void AsyncBlockerManager::unlock()
    {
        impl()->unlock();
    }
    
    auto AsyncBlockerManager::add_blocker(int priority, close_chan closer) -> asio::awaitable<AsyncBlocker::Ptr>
    {
        auto block_ptr = co_await impl()->add_blocker(priority, closer);
        co_return std::make_shared<AsyncBlocker>(block_ptr);
    }
} // namespace cfgo

