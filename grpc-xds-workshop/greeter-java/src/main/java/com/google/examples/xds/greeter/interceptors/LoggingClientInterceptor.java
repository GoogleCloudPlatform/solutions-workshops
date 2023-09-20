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

package com.google.examples.xds.greeter.interceptors;

import com.google.examples.xds.greeter.logging.MessagePrinter;
import com.google.rpc.ErrorInfo;
import io.grpc.CallOptions;
import io.grpc.Channel;
import io.grpc.ClientCall;
import io.grpc.ClientInterceptor;
import io.grpc.ForwardingClientCall;
import io.grpc.ForwardingClientCallListener;
import io.grpc.Metadata;
import io.grpc.MethodDescriptor;
import io.grpc.Status;
import io.grpc.examples.helloworld.HelloReply;
import io.grpc.examples.helloworld.HelloRequest;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

/** Logs requests sent and responses received by a client. */
public class LoggingClientInterceptor implements ClientInterceptor {

  private static final Logger LOG = LoggerFactory.getLogger(LoggingClientInterceptor.class);

  private final MessagePrinter messagePrinter;

  /** Create a client interceptor configured to log proto messages as JSON. */
  public LoggingClientInterceptor() {
    this.messagePrinter =
        new MessagePrinter(
            HelloRequest.getDescriptor(), HelloReply.getDescriptor(), ErrorInfo.getDescriptor());
  }

  @Override
  public <ReqT, RespT> ClientCall<ReqT, RespT> interceptCall(
      MethodDescriptor<ReqT, RespT> method, CallOptions callOptions, Channel next) {
    return new ForwardingClientCall.SimpleForwardingClientCall<>(
        next.newCall(method, callOptions)) {
      @Override
      public void sendMessage(ReqT message) {
        if (LOG.isDebugEnabled()) {
          LOG.debug(
              "Sending request {}:\n{}",
              message.getClass().getCanonicalName(),
              messagePrinter.print(message));
        }
        super.sendMessage(message);
      }

      @Override
      public void start(Listener<RespT> responseListener, Metadata headers) {
        super.start(
            new ForwardingClientCallListener.SimpleForwardingClientCallListener<>(
                responseListener) {
              @Override
              public void onMessage(RespT message) {
                if (LOG.isDebugEnabled()) {
                  LOG.debug(
                      "Received response {}:\n{}",
                      message.getClass().getCanonicalName(),
                      messagePrinter.print(message));
                }
                super.onMessage(message);
              }

              @Override
              public void onClose(Status status, Metadata trailers) {
                if (!status.isOk()) {
                  LOG.error(
                      "RPC status code: {} description: {}",
                      status.getCode(),
                      status.getDescription(),
                      status.asException(trailers));
                } else {
                  LOG.debug(
                      "RPC status code: {} description: {}",
                      status.getCode(),
                      status.getDescription());
                }
                super.onClose(status, trailers);
              }
            },
            headers);
      }
    };
  }
}
