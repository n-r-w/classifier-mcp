package server

// inputSchema describes the inline request, including exclusive question variants and closed defined objects.
const inputSchema = `{
  "type": "object",
  "properties": {
    "objects": {
      "type": "array",
      "minItems": 1,
      "items": {
        "type": "object",
        "properties": {
          "id": {
            "type": "string",
            "minLength": 1
          },
          "source": {
            "type": "object",
            "properties": {
              "type": {
                "type": "string",
                "enum": [
                  "text"
                ]
              },
              "text": {
                "type": "string"
              }
            },
            "required": [
              "type",
              "text"
            ],
            "additionalProperties": false
          }
        },
        "required": [
          "id",
          "source"
        ],
        "additionalProperties": false
      }
    },
    "task": {
      "type": "string",
      "minLength": 1
    },
    "questions": {
      "type": "object",
      "additionalProperties": {
        "oneOf": [
          {
            "type": "object",
            "properties": {
              "type": {
                "type": "string",
                "enum": [
                  "choice"
                ]
              },
              "instructions": {
                "anyOf": [
                  {
                    "type": "string"
                  },
                  {
                    "type": "object"
                  },
                  {
                    "type": "array"
                  }
                ]
              },
              "criteria": {
                "type": "object",
                "additionalProperties": {
                  "anyOf": [
                    {
                      "type": "string"
                    },
                    {
                      "type": "object"
                    },
                    {
                      "type": "array"
                    },
                    {
                      "type": "null"
                    }
                  ]
                },
                "minProperties": 1,
                "propertyNames": {
                  "type": "string",
                  "minLength": 1
                }
              }
            },
            "required": [
              "type",
              "instructions",
              "criteria"
            ],
            "additionalProperties": false
          },
          {
            "type": "object",
            "properties": {
              "type": {
                "type": "string",
                "enum": [
                  "noul"
                ]
              },
              "instructions": {
                "anyOf": [
                  {
                    "type": "string"
                  },
                  {
                    "type": "object"
                  },
                  {
                    "type": "array"
                  }
                ]
              },
              "criteria": {
                "type": "object",
                "properties": {
                  "true": {
                    "anyOf": [
                      {
                        "type": "string"
                      },
                      {
                        "type": "object"
                      },
                      {
                        "type": "array"
                      }
                    ]
                  },
                  "false": {
                    "anyOf": [
                      {
                        "type": "string"
                      },
                      {
                        "type": "object"
                      },
                      {
                        "type": "array"
                      }
                    ]
                  }
                },
                "required": [
                  "true",
                  "false"
                ],
                "additionalProperties": false
              }
            },
            "required": [
              "type",
              "instructions"
            ],
            "additionalProperties": false
          },
          {
            "type": "object",
            "properties": {
              "type": {
                "type": "string",
                "enum": [
                  "score"
                ]
              },
              "instructions": {
                "anyOf": [
                  {
                    "type": "string"
                  },
                  {
                    "type": "object"
                  },
                  {
                    "type": "array"
                  }
                ]
              },
              "criteria": {
                "type": "array",
                "items": {
                  "anyOf": [
                    {
                      "type": "string"
                    },
                    {
                      "type": "object"
                    },
                    {
                      "type": "array"
                    }
                  ]
                },
                "minItems": 1
              }
            },
            "required": [
              "type",
              "instructions",
              "criteria"
            ],
            "additionalProperties": false
          }
        ]
      },
      "minProperties": 1,
      "propertyNames": {
        "type": "string",
        "minLength": 1
      }
    },
    "result_mode": {
      "type": "string",
      "enum": [
        "compact",
        "full"
      ]
    }
  },
  "required": [
    "objects",
    "task",
    "questions"
  ],
  "additionalProperties": false
}`

// outputSchema describes success and atomic failure entries, with compact and full assessment alternatives.
const outputSchema = `{
  "type": "object",
  "properties": {
    "results": {
      "type": "array",
      "minItems": 1,
      "items": {
        "oneOf": [
          {
            "type": "object",
            "properties": {
              "id": {
                "type": "string"
              },
              "status": {
                "type": "string",
                "enum": [
                  "ok"
                ]
              },
              "model": {
                "type": "string",
                "minLength": 1
              },
              "answers": {
                "type": "object",
                "additionalProperties": {
                  "oneOf": [
                    {
                      "type": "object",
                      "properties": {
                        "type": {
                          "type": "string",
                          "enum": [
                            "choice"
                          ]
                        },
                        "choice": {
                          "type": "string"
                        },
                        "probability": {
                          "type": "number",
                          "minimum": 0,
                          "maximum": 1
                        },
                        "confidence": {
                          "type": "number",
                          "minimum": 0,
                          "maximum": 1
                        }
                      },
                      "required": [
                        "type",
                        "choice",
                        "probability"
                      ],
                      "additionalProperties": false
                    },
                    {
                      "type": "object",
                      "properties": {
                        "type": {
                          "type": "string",
                          "enum": [
                            "choice"
                          ]
                        },
                        "choice": {
                          "type": "string"
                        },
                        "probability": {
                          "type": "number",
                          "minimum": 0,
                          "maximum": 1
                        },
                        "confidence": {
                          "type": "number",
                          "minimum": 0,
                          "maximum": 1
                        },
                        "probabilities": {
                          "type": "object",
                          "additionalProperties": {
                            "type": "number",
                            "minimum": 0,
                            "maximum": 1
                          },
                          "minProperties": 1,
                          "propertyNames": {
                            "type": "string",
                            "minLength": 1
                          }
                        }
                      },
                      "required": [
                        "type",
                        "choice",
                        "probability",
                        "probabilities"
                      ],
                      "additionalProperties": false
                    },
                    {
                      "type": "object",
                      "properties": {
                        "type": {
                          "type": "string",
                          "enum": [
                            "score"
                          ]
                        },
                        "score": {
                          "type": "number",
                          "minimum": 0
                        },
                        "confidence": {
                          "type": "number",
                          "minimum": 0,
                          "maximum": 1
                        }
                      },
                      "required": [
                        "type",
                        "score"
                      ],
                      "additionalProperties": false
                    },
                    {
                      "type": "object",
                      "properties": {
                        "type": {
                          "type": "string",
                          "enum": [
                            "score"
                          ]
                        },
                        "score": {
                          "type": "number",
                          "minimum": 0
                        },
                        "confidence": {
                          "type": "number",
                          "minimum": 0,
                          "maximum": 1
                        },
                        "probabilities": {
                          "type": "object",
                          "additionalProperties": {
                            "type": "number",
                            "minimum": 0,
                            "maximum": 1
                          },
                          "minProperties": 1,
                          "propertyNames": {
                            "type": "string",
                            "minLength": 1
                          }
                        },
                        "legend": {
                          "type": "object",
                          "additionalProperties": {
                            "anyOf": [
                              {
                                "type": "string"
                              },
                              {
                                "type": "object"
                              },
                              {
                                "type": "array"
                              }
                            ]
                          },
                          "minProperties": 1,
                          "propertyNames": {
                            "type": "string",
                            "minLength": 1
                          }
                        }
                      },
                      "required": [
                        "type",
                        "score",
                        "probabilities",
                        "legend"
                      ],
                      "additionalProperties": false
                    },
                    {
                      "type": "object",
                      "properties": {
                        "type": {
                          "type": "string",
                          "enum": [
                            "noul"
                          ]
                        },
                        "noul": {
                          "type": "number",
                          "minimum": 0,
                          "maximum": 1
                        }
                      },
                      "required": [
                        "type",
                        "noul"
                      ],
                      "additionalProperties": false
                    }
                  ]
                },
                "minProperties": 1,
                "propertyNames": {
                  "type": "string",
                  "minLength": 1
                }
              },
              "usage": {
                "type": "object",
                "properties": {
                  "input_tokens": {
                    "type": "integer",
                    "minimum": 0
                  },
                  "output_tokens": {
                    "type": "integer",
                    "minimum": 0
                  },
                  "cost": {
                    "type": "number",
                    "minimum": 0
                  }
                },
                "required": [],
                "additionalProperties": false,
                "minProperties": 1
              }
            },
            "required": [
              "id",
              "status",
              "model",
              "answers"
            ],
            "additionalProperties": false
          },
          {
            "type": "object",
            "properties": {
              "id": {
                "type": "string"
              },
              "status": {
                "type": "string",
                "enum": [
                  "error"
                ]
              },
              "error": {
                "type": "object",
                "properties": {
                  "code": {
                    "type": "string",
                    "enum": [
                      "upstream_error",
                      "request_failed",
                      "invalid_response",
                      "canceled"
                    ]
                  },
                  "operation": {
                    "type": "string",
                    "enum": [
                      "classify"
                    ]
                  },
                  "message": {
                    "type": "string"
                  },
                  "http_status": {
                    "type": "integer"
                  },
                  "upstream_body": {
                    "type": "string"
                  },
                  "upstream_request_id": {
                    "type": "string"
                  },
                  "attempts": {
                    "type": "integer",
                    "minimum": 1
                  },
                  "retry_after_seconds": {
                    "type": "number",
                    "minimum": 0
                  }
                },
                "required": [
                  "code",
                  "operation",
                  "message"
                ],
                "additionalProperties": false
              },
              "usage": {
                "type": "object",
                "properties": {
                  "input_tokens": {
                    "type": "integer",
                    "minimum": 0
                  },
                  "output_tokens": {
                    "type": "integer",
                    "minimum": 0
                  },
                  "cost": {
                    "type": "number",
                    "minimum": 0
                  }
                },
                "required": [],
                "additionalProperties": false,
                "minProperties": 1
              }
            },
            "required": [
              "id",
              "status",
              "error"
            ],
            "additionalProperties": false
          }
        ]
      }
    }
  },
  "required": [
    "results"
  ],
  "additionalProperties": false
}`

// Shared JSON field names keep the result serializers aligned with the MCP schema.
const (
	fieldID            = "id"
	fieldStatus        = "status"
	fieldType          = "type"
	fieldConfidence    = "confidence"
	fieldProbabilities = "probabilities"
	fieldUsage         = "usage"
	statusError        = "error"
)
