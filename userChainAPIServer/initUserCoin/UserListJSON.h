#pragma once
#include <stdint.h>
#include <stdlib.h>

#ifdef __cplusplus
extern "C"
{
#endif /* __cplusplus */

    void addUser(int puid, const char *puname, const char *coin);
    const char *getUserListJson(int lastUserId, const char *coin);
    int64_t getUserUpdateTime(const char *puname, const char *coin);

#ifdef __cplusplus
}
#endif /* __cplusplus */
