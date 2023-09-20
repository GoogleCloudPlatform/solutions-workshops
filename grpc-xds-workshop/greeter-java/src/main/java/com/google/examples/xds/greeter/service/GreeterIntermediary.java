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

import com.google.protobuf.Any;
import com.google.rpc.ErrorInfo;
import com.google.rpc.Status;
import io.grpc.examples.helloworld.GreeterGrpc.GreeterImplBase;
import io.grpc.examples.helloworld.HelloReply;
import io.grpc.examples.helloworld.HelloRequest;
import io.grpc.protobuf.StatusProto;
import io.grpc.stub.StreamObserver;
import org.jetbrains.annotations.NotNull;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

/**
 * Greeter implementation which sends a request to the next service (<code>NEXT_HOP</code>) and
 * forwards the response, after appending its own hostname.
 *
 * @see GreeterLeaf for the other implementation.
 * @see GreeterClient for the client implementation.
 */
public class GreeterIntermediary extends GreeterImplBase {

  private static final Logger LOG = LoggerFactory.getLogger(GreeterIntermediary.class);

  private final String name;
  private final GreeterClient client;

  /**
   * Creates a greeter intermediary.
   *
   * @param name of this intermediary
   * @param client to connect to the next hop
   */
  public GreeterIntermediary(@NotNull String name, @NotNull GreeterClient client) {
    this.name = name;
    this.client = client;
  }

  @Override
  public void sayHello(HelloRequest req, StreamObserver<HelloReply> responseObserver) {
    LOG.info("Received request to greet {}, forwarding to the next hop.", req.getName());
    try {
      String intermediaryMessage = client.sayHello(req.getName());
      var reply = HelloReply.newBuilder().setMessage(intermediaryMessage + ", via " + name).build();
      responseObserver.onNext(reply);
      responseObserver.onCompleted();
    } catch (GreeterException e) {
      LOG.warn("Greeting request failed, returning error code internal.", e);
      responseObserver.onError(
          StatusProto.toStatusRuntimeException(
              createStatus(io.grpc.Status.Code.INTERNAL, "greeter request failed")));
    }
  }

  /**
   * Create a {@link com.google.rpc.Status} instance with an attached {@link
   * com.google.rpc.ErrorInfo} instance. See the <a
   * href="https://cloud.google.com/apis/design/errors">error model for Google APIs</a>.
   *
   * @param code Provides the numeric <code>code</code> and text constant <code>reason</code>.
   * @param message Developer-facing message.
   * @return Status instance that can be turned into a {@link io.grpc.StatusRuntimeException}.
   */
  private static Status createStatus(io.grpc.Status.Code code, String message) {
    var errorInfo =
        ErrorInfo.newBuilder().setReason(code.name()).setDomain("greeter.example.com").build();
    return Status.newBuilder()
        .setCode(code.value())
        .setMessage(message)
        .addDetails(Any.pack(errorInfo))
        .build();
  }
}
