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

package com.google.examples.xds.greeter.logging;

import com.google.protobuf.Descriptors.Descriptor;
import com.google.protobuf.InvalidProtocolBufferException;
import com.google.protobuf.MessageOrBuilder;
import com.google.protobuf.TextFormat;
import com.google.protobuf.util.JsonFormat;
import com.google.protobuf.util.JsonFormat.TypeRegistry;
import org.jetbrains.annotations.NotNull;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

/**
 * Turns proto messages into strings, as JSON if the type is registered, otherwise as plain text.
 */
public class MessagePrinter {

  private static final Logger LOG = LoggerFactory.getLogger(MessagePrinter.class);

  private final JsonFormat.Printer jsonPrinter;
  private final TextFormat.Printer textPrinter;

  /**
   * Creates a message printer.
   *
   * @param messageTypes proto descriptors for messages that should be printed as JSON strings.
   */
  public MessagePrinter(Descriptor... messageTypes) {
    TypeRegistry.Builder typeRegistryBuilder = TypeRegistry.newBuilder();
    for (Descriptor messageType : messageTypes) {
      typeRegistryBuilder.add(messageType);
    }
    this.jsonPrinter = JsonFormat.printer().usingTypeRegistry(typeRegistryBuilder.build());
    this.textPrinter = TextFormat.printer();
  }

  /**
   * Create a string representation of the proto message.
   *
   * @param msg the message to turn into a string, should be implementation of {@code
   *     com.google.protobuf.MessageOrBuilder}
   * @return string representation of the message
   */
  @NotNull
  public String print(Object msg) {
    if (msg == null) {
      LOG.warn("Message is null");
      return "null";
    }
    if (!MessageOrBuilder.class.isAssignableFrom(msg.getClass())) {
      LOG.warn(
          "Message has unexpected type {}, using toString()", msg.getClass().getCanonicalName());
      return msg.toString();
    }
    MessageOrBuilder message = (MessageOrBuilder) msg;
    try {
      return jsonPrinter.print(message);
    } catch (InvalidProtocolBufferException e) {
      LOG.warn("Could not convert message to JSON, using text printer instead: {}", e.getMessage());
      return textPrinter.printToString(message);
    }
  }
}
