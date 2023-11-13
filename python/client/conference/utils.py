def splitUrl(url: str):
    spos = url.find('://')
    startPos = 0
    if spos != -1:
        startPos = spos + 3
    pos = url.find('/', startPos)
    if pos != -1:
        return url[0:pos], url[pos:]
    else:
        return url, '/'
