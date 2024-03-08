// Copyright 2023 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package com.google.examples.xds.controlplane.logging;

import java.util.logging.Filter;
import java.util.logging.LogRecord;
import java.util.regex.Pattern;

/** Masks access tokens in log records. */
@SuppressWarnings("unused")
public class AccessTokenMaskingFilter implements Filter {

  private static final Pattern ACCESS_TOKEN_PATTERN =
      Pattern.compile("([Aa]uthorization:?\\s*)(Bearer )?(.+)");

  @Override
  public boolean isLoggable(LogRecord logRecord) {
    if (logRecord == null || logRecord.getMessage() == null) {
      return false;
    }
    var matcher = ACCESS_TOKEN_PATTERN.matcher(logRecord.getMessage());
    var message = matcher.replaceFirst("$1$2REDACTED");
    logRecord.setMessage(message);
    return true;
  }
}
