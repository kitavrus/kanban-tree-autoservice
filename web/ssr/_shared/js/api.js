/* KT.api — fetch-обёртка для JSON API (/api/v1/...): same-origin cookie sid,
   CSRF-токен из <meta name="csrf-token"> заголовком X-Csrf-Token на мутациях,
   JSON в обе стороны. Ошибка → throw ApiError {status, code, message, fields}
   (wire-формат pkg/errorspkg; 400 → details.fields[]). */
(function () {
  'use strict';
  var KT = window.KT = window.KT || {};

  function ApiError(status, body) {
    var err = (body && body.error) || body || {};
    this.name = 'ApiError';
    this.status = status;
    this.code = err.code || '';
    this.message = err.message || ('HTTP ' + status);
    this.details = err.details || null;
    this.fields = (err.details && (err.details.fields || err.details.Fields)) || [];
  }
  ApiError.prototype = Object.create(Error.prototype);

  function csrfToken() {
    var m = document.querySelector('meta[name="csrf-token"]');
    return m ? m.content : '';
  }

  // opts.signal — AbortSignal для отменяемых фоновых запросов (доска отменяет их на
  // pagehide/уходе со страницы, чтобы не долетал ответ уже после logout — BD-021).
  function request(method, url, body, opts) {
    var headers = { Accept: 'application/json' };
    var init = { method: method, headers: headers, credentials: 'same-origin' };
    if (opts && opts.signal) init.signal = opts.signal;
    if (method !== 'GET' && method !== 'HEAD') {
      headers['X-Csrf-Token'] = csrfToken();
      if (body instanceof FormData) init.body = body;
      else if (body !== undefined) { headers['Content-Type'] = 'application/json'; init.body = JSON.stringify(body); }
    }
    return fetch(url, init).then(function (res) {
      var ct = res.headers.get('Content-Type') || '';
      var parse = ct.indexOf('application/json') >= 0 ? res.json() : res.text();
      return parse.then(function (data) {
        if (!res.ok) throw new ApiError(res.status, data);
        return data;
      });
    });
  }

  KT.ApiError = ApiError;
  KT.api = {
    get: function (url, opts) { return request('GET', url, undefined, opts); },
    post: function (url, body, opts) { return request('POST', url, body, opts); },
    put: function (url, body, opts) { return request('PUT', url, body, opts); },
    patch: function (url, body, opts) { return request('PATCH', url, body, opts); },
    del: function (url, opts) { return request('DELETE', url, undefined, opts); },
    // DELETE с JSON-телом — мягкое удаление с причиной {reason} (soft-delete-core DS-028).
    delWith: function (url, body, opts) { return request('DELETE', url, body, opts); }
  };
})();
