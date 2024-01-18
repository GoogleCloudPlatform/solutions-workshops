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

package com.google.examples.xds.greeter.service;

import com.google.examples.xds.greeter.interceptors.LoggingClientInterceptor;
import io.grpc.Grpc;
import io.grpc.InsecureChannelCredentials;
import io.grpc.ManagedChannel;
import io.grpc.StatusRuntimeException;
import io.grpc.examples.helloworld.GreeterGrpc;
import io.grpc.examples.helloworld.HelloReply;
import io.grpc.examples.helloworld.HelloRequest;
import io.grpc.xds.XdsChannelCredentials;
import java.util.concurrent.TimeUnit;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

/** Implements a client for the <code>helloworld.Greeter</code> service. */
public class GreeterClient {
  private static final Logger LOG = LoggerFactory.getLogger(GreeterClient.class);

  private final GreeterGrpc.GreeterBlockingStub blockingStub;

  /** Creates a client for the <code>helloworld.Greeter</code> service. */
  public GreeterClient(String target, boolean useXds) {
    var channelCredentials = InsecureChannelCredentials.create();
    if (useXds) {
      channelCredentials = XdsChannelCredentials.create(InsecureChannelCredentials.create());
    }
    ManagedChannel channel =
        Grpc.newChannelBuilder(target, channelCredentials)
            // Idle timeout >= 30 days to disable idle timeout.
            // See https://github.com/grpc/grpc-java/issues/9632
            .idleTimeout(365, TimeUnit.DAYS)
            .intercept(new LoggingClientInterceptor())
            .build();
    addChannelShutdownHook(channel);
    this.blockingStub = GreeterGrpc.newBlockingStub(channel);
  }

  /**
   * Calls the <code>sayHello</code> method of the <code>helloworld.Greeter</code> service.
   *
   * @param name to greet
   * @return greeting
   */
  public String sayHello(String name) {
    var request = HelloRequest.newBuilder().setName(name).build();
    try {
      HelloReply reply = blockingStub.sayHello(request);
      return reply.getMessage();
    } catch (StatusRuntimeException e) {
      LOG.error(
          "SayHello request failed with status={}, message={}, trailers={}",
          e.getStatus(),
          e.getMessage(),
          e.getTrailers(),
          e);
      throw new GreeterException("SayHello request failed", e);
    }
  }

  private void addChannelShutdownHook(ManagedChannel channel) {
    Runtime.getRuntime()
        .addShutdownHook(
            new Thread(
                () -> {
                  // Start graceful shutdown.
                  channel.shutdown();
                  try {
                    // Wait for RPCs to complete processing.
                    if (!channel.awaitTermination(5, TimeUnit.SECONDS)) {
                      // That was plenty of time. Let's cancel the remaining RPCs.
                      channel.shutdownNow();
                      // shutdownNow isn't instantaneous, so give a bit of time to clean resources
                      // up gracefully. Normally this will be well under a second.
                      channel.awaitTermination(1, TimeUnit.SECONDS);
                    }
                  } catch (InterruptedException ex) {
                    channel.shutdownNow();
                  }
                }));
  }
}
