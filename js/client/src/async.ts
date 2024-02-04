
type ExtractAsyncIterableT<T extends AsyncIterableIterator<any>> = T extends AsyncIterableIterator<infer X> ? X : never;

export async function* combineAsyncIterable<T extends AsyncIterableIterator<any>[]>(iterables: readonly [...T]): AsyncIterableIterator<ExtractAsyncIterableT<T[number]>> {
    const asyncIterators = Array.from(iterables, o => o[Symbol.asyncIterator]());
    const results: T[number][] = [];
    let count = asyncIterators.length;
    function getNext(asyncIterator: typeof asyncIterators[number], index: number) {
        return asyncIterator.next().then(result => ({
            index,
            result,
        }));
    }
    const nextPromises = asyncIterators.map(getNext);
    const never: typeof nextPromises[number] = new Promise(() => {});
    try {
        while (count) {
            const {index, result} = await Promise.race(nextPromises);
            if (result.done) {
                nextPromises[index] = never;
                results[index] = result.value;
                count--;
            } else {
                nextPromises[index] = getNext(asyncIterators[index], index);
                yield result.value;
            }
        }
    } finally {
        for (const [index, iterator] of asyncIterators.entries())
            if (nextPromises[index] != never && iterator.return != null)
                iterator.return();
        // no await here - see https://github.com/tc39/proposal-async-iteration/issues/126
    }
    return results;
}

