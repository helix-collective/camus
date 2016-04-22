worker_processes auto;
pid PID_FILE;

daemon off;

events {
  worker_connections 768;
  # multi_accept on;
}

http {

  access_log /dev/null;
  error_log  error.log;  # TODO(dan): Plumb error logs offsite instead of on disk
  
  # Conceivably useful.  Outer server should set X-Real-IP / X-Forwarded-For
  proxy_set_header X-Inner-Real-IP $remote_addr;
  proxy_read_timeout 600;
  
  # Cache proxied backend results
  proxy_cache_path proxy_cache 
      keys_zone=appzone:10m 
      levels=1:2 max_size=1g inactive=60m;
  proxy_cache appzone;
  add_header X-Cache-Status $upstream_cache_status;
  
  # Client request body control
  proxy_request_buffering on;
  client_body_temp_path client_temp 1 2;
  client_max_body_size 20M;
  
  # Proxy response control
  proxy_buffering on;
  proxy_temp_path proxy_temp 1 2;
  
  # Serve "foo" with "foo.gz" wherever the latter exists (and browser supports it)
  gzip_static on;
  
  # Add 'Vary: Accept-Encoding' header for static assets that have gzip alternatives
  gzip_vary on;
  
  # Assume textual mime-types are utf8 and set charset header accordingly
  charset utf-8;
  
  
  server {
    listen %PORT% default_server;
    
    index index.html;
    root %ROOT%;
    
    # TODO(dan): Work out how to add this as a default for static files
    # without affecting proxied requests. Perhaps we just need to be explicit
    # about all proxied endpoints instead of using try_files ... @backend
    # but rather have static files out of 'location /' be the last fallback. 
    # 
    #   add_header Cache-Control "no-cache, no-store, must-revalidate";
    
    try_files /$uri @backend;
    
    
    # Anything starting with /static that also contains what appears to be a sha hash
    # will be cached for a year.
    # we need some sane prefix e.g. /static otherwise the sha match could hit
    # cause requests that would otherwise be proxied to be treated as static,
    # due to the location match here.
    # if a proxied request contains an infinicacheable result, the origin server
    # may set appropriate cache-control headers (and our nginx will cache the
    # result too, reducing load on the app server in addition to leveraging browser caches)
    location ~ "^/static.*[a-f0-9]{40}" {
      # 1 year
      add_header Cache-Control "public, max-age=31536000";
    }
    
    # Explicit proxy /_ regardless of contents of docroot.
    # Backend server owns /_ and no time spent accessing disk to see if
    # files are present.
    location /_ {
      proxy_pass http://127.0.0.1:%FORWARD_PORT%/_;
    }
    
    location @backend {
      proxy_pass http://127.0.0.1:%FORWARD_PORT%; # no _ prefix
    }
        
      # Backend owns the index page, so explicit proxy / 
      location = / {
        proxy_pass http://127.0.0.1:%FORWARD_PORT%/;
      }
  }

  #
# From
# https://github.com/h5bp/server-configs-nginx/blob/master/mime.types
#

types {

  # Data interchange

    application/atom+xml                  atom;
    application/json                      json map topojson;
    application/ld+json                   jsonld;
    application/rss+xml                   rss;
    application/vnd.geo+json              geojson;
    application/xml                       rdf xml;


  # JavaScript

    # Normalize to standard type.
    # https://tools.ietf.org/html/rfc4329#section-7.2
    application/javascript                js;


  # Manifest files

    application/manifest+json             webmanifest;
    application/x-web-app-manifest+json   webapp;
    text/cache-manifest                   appcache;


  # Media files

    audio/midi                            mid midi kar;
    audio/mp4                             aac f4a f4b m4a;
    audio/mpeg                            mp3;
    audio/ogg                             oga ogg opus;
    audio/x-realaudio                     ra;
    audio/x-wav                           wav;
    image/bmp                             bmp;
    image/gif                             gif;
    image/jpeg                            jpeg jpg;
    image/png                             png;
    image/svg+xml                         svg svgz;
    image/tiff                            tif tiff;
    image/vnd.wap.wbmp                    wbmp;
    image/webp                            webp;
    image/x-jng                           jng;
    video/3gpp                            3gp 3gpp;
    video/mp4                             f4p f4v m4v mp4;
    video/mpeg                            mpeg mpg;
    video/ogg                             ogv;
    video/quicktime                       mov;
    video/webm                            webm;
    video/x-flv                           flv;
    video/x-mng                           mng;
    video/x-ms-asf                        asf asx;
    video/x-ms-wmv                        wmv;
    video/x-msvideo                       avi;

    # Serving `.ico` image files with a different media type
    # prevents Internet Explorer from displaying then as images:
    # https://github.com/h5bp/html5-boilerplate/commit/37b5fec090d00f38de64b591bcddcb205aadf8ee

    image/x-icon                          cur ico;


  # Microsoft Office

    application/msword                                                         doc;
    application/vnd.ms-excel                                                   xls;
    application/vnd.ms-powerpoint                                              ppt;
    application/vnd.openxmlformats-officedocument.wordprocessingml.document    docx;
    application/vnd.openxmlformats-officedocument.spreadsheetml.sheet          xlsx;
    application/vnd.openxmlformats-officedocument.presentationml.presentation  pptx;


  # Web fonts

    application/font-woff                 woff;
    application/font-woff2                woff2;
    application/vnd.ms-fontobject         eot;

    # Browsers usually ignore the font media types and simply sniff
    # the bytes to figure out the font type.
    # https://mimesniff.spec.whatwg.org/#matching-a-font-type-pattern
    #
    # However, Blink and WebKit based browsers will show a warning
    # in the console if the following font types are served with any
    # other media types.

    application/x-font-ttf                ttc ttf;
    font/opentype                         otf;


  # Other

    application/java-archive              ear jar war;
    application/mac-binhex40              hqx;
    application/octet-stream              bin deb dll dmg exe img iso msi msm msp safariextz;
    application/pdf                       pdf;
    application/postscript                ai eps ps;
    application/rtf                       rtf;
    application/vnd.google-earth.kml+xml  kml;
    application/vnd.google-earth.kmz      kmz;
    application/vnd.wap.wmlc              wmlc;
    application/x-7z-compressed           7z;
    application/x-bb-appworld             bbaw;
    application/x-bittorrent              torrent;
    application/x-chrome-extension        crx;
    application/x-cocoa                   cco;
    application/x-java-archive-diff       jardiff;
    application/x-java-jnlp-file          jnlp;
    application/x-makeself                run;
    application/x-opera-extension         oex;
    application/x-perl                    pl pm;
    application/x-pilot                   pdb prc;
    application/x-rar-compressed          rar;
    application/x-redhat-package-manager  rpm;
    application/x-sea                     sea;
    application/x-shockwave-flash         swf;
    application/x-stuffit                 sit;
    application/x-tcl                     tcl tk;
    application/x-x509-ca-cert            crt der pem;
    application/x-xpinstall               xpi;
    application/xhtml+xml                 xhtml;
    application/xslt+xml                  xsl;
    application/zip                       zip;
    text/css                              css;
    text/html                             htm html shtml;
    text/mathml                           mml;
    text/plain                            txt;
    text/vcard                            vcard vcf;
    text/vnd.rim.location.xloc            xloc;
    text/vnd.sun.j2me.app-descriptor      jad;
    text/vnd.wap.wml                      wml;
    text/vtt                              vtt;
    text/x-component                      htc;

}

}


