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

package com.google.examples.xds.greeter;

import com.google.examples.xds.greeter.config.ServerConfig;
import com.google.examples.xds.greeter.server.Server;
import java.net.URL;

/** Contains the entry point for the gRPC greeter server. */
@SuppressWarnings("CatchAndPrintStackTrace")
public class Main {

  // Set up logging if not already configured.
  static {
    try {
      if (System.getProperty("java.util.logging.config.class") == null
          && System.getProperty("java.util.logging.config.file") == null) {
        URL resource = Main.class.getClassLoader().getResource("logging.properties");
        if (resource != null) {
          System.setProperty("java.util.logging.config.file", resource.getFile());
        }
      }
    } catch (Exception e) {
      //noinspection CallToPrintStackTrace
      e.printStackTrace();
    }
  }

  /** Entry point. */
  public static void main(String[] args) throws Exception {
    new Server().run(new ServerConfig());
  }
}
