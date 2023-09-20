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

import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.junit.jupiter.api.Assertions.fail;

import com.google.examples.xds.greeter.service.GreeterLeaf;
import io.grpc.examples.helloworld.HelloReply;
import io.grpc.examples.helloworld.HelloRequest;
import io.grpc.stub.StreamObserver;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.TimeUnit;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.ValueSource;

/** Tests for GreeterLeaf. */
public class GreeterLeafTest {

  /** Test error handling. */
  @ParameterizedTest
  @ValueSource(strings = {"", "foo"})
  public void sayHello(String name) throws Exception {
    final var onNextLatch = new CountDownLatch(1);
    final var onCompletedLatch = new CountDownLatch(1);
    new GreeterLeaf("name")
        .sayHello(
            HelloRequest.newBuilder().setName(name).build(),
            new StreamObserver<>() {
              @Override
              public void onNext(HelloReply reply) {
                assertNotNull(reply);
                assertTrue(reply.getMessage().contains(name));
                onNextLatch.countDown();
              }

              @Override
              public void onError(Throwable t) {
                fail();
              }

              @Override
              public void onCompleted() {
                try {
                  assertTrue(onNextLatch.await(3, TimeUnit.SECONDS));
                } catch (InterruptedException e) {
                  fail(e);
                }
                onCompletedLatch.countDown();
              }
            });
    assertTrue(onCompletedLatch.await(3, TimeUnit.SECONDS));
  }
}
